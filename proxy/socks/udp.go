package socks

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/xjasonlyu/tun2socks/common/dns"
	"github.com/xjasonlyu/tun2socks/common/log"
	"github.com/xjasonlyu/tun2socks/common/lsof"
	"github.com/xjasonlyu/tun2socks/common/stats"
	"github.com/xjasonlyu/tun2socks/core"
)

type udpHandler struct {
	proxyHost string
	proxyPort uint16
	timeout   time.Duration

	remoteAddrMap       sync.Map
	remoteConnMap       sync.Map
	remotePacketConnMap sync.Map

	fakeDns       dns.FakeDns
	sessionStater stats.SessionStater
}

func NewUDPHandler(proxyHost string, proxyPort uint16, timeout time.Duration, fakeDns dns.FakeDns, sessionStater stats.SessionStater) core.UDPConnHandler {
	return &udpHandler{
		proxyHost:     proxyHost,
		proxyPort:     proxyPort,
		fakeDns:       fakeDns,
		sessionStater: sessionStater,
		timeout:       timeout,
	}
}

func (h *udpHandler) handleTCP(conn core.UDPConn, c net.Conn) {
	for {
		// clear timeout settings
		c.SetDeadline(time.Time{})
		_, err := c.Read(make([]byte, 1))
		if err != nil {
			if err == io.EOF {
				log.Warnf("UDP associate to %v closed by remote", c.RemoteAddr())
			}
			h.Close(conn)
			return
		}
	}
}

func (h *udpHandler) fetchUDPInput(conn core.UDPConn, input net.PacketConn) {
	buf := core.NewBytes(core.BufSize)

	defer func() {
		h.Close(conn)
		core.FreeBytes(buf)
	}()

	for {
		input.SetDeadline(time.Now().Add(h.timeout))
		n, _, err := input.ReadFrom(buf)
		if err != nil {
			if err, ok := err.(net.Error); !ok && !err.Timeout() {
				log.Warnf("read remote failed: %v", err)
			}
			return
		}

		if n < 4 {
			log.Warnf("short udp packet length: %d", n)
			return
		}

		addr := SplitAddr(buf[3:])
		resolvedAddr, err := net.ResolveUDPAddr("udp", addr.String())
		if err != nil {
			return
		}

		if _, err := conn.WriteFrom(buf[int(3+len(addr)):n], resolvedAddr); err != nil {
			log.Warnf("write local failed: %v", err)
			return
		}
	}
}

func (h *udpHandler) Connect(conn core.UDPConn, target *net.UDPAddr) error {
	if target == nil {
		return h.connectInternal(conn, "")
	}

	// Replace with a domain name if target address IP is a fake IP.
	var targetHost = target.IP.String()
	if h.fakeDns != nil {
		if host, exist := h.fakeDns.IPToHost(target.IP); exist {
			targetHost = host
		}
	}
	targetAddr := net.JoinHostPort(targetHost, strconv.Itoa(target.Port))
	if len(targetAddr) == 0 {
		return errors.New("target address is empty")
	}

	return h.connectInternal(conn, targetAddr)
}

func (h *udpHandler) connectInternal(conn core.UDPConn, targetAddr string) error {
	remoteConn, err := net.DialTimeout("tcp", core.ParseTCPAddr(h.proxyHost, h.proxyPort).String(), 4*time.Second)
	if err != nil {
		return err
	}
	remoteConn.SetDeadline(time.Now().Add(5 * time.Second))

	// send VER, NMETHODS, METHODS
	if _, err := remoteConn.Write([]byte{socks5Version, 1, 0}); err != nil {
		return err
	}

	buf := make([]byte, MaxAddrLen)
	// read VER METHOD
	if _, err := io.ReadFull(remoteConn, buf[:2]); err != nil {
		return err
	}

	destination := ParseAddr(targetAddr)
	// write VER CMD RSV ATYP DST.ADDR DST.PORT
	if _, err := remoteConn.Write(append([]byte{socks5Version, socks5UDPAssociate, 0}, destination...)); err != nil {
		return err
	}

	// read VER REP RSV ATYP BND.ADDR BND.PORT
	if _, err := io.ReadFull(remoteConn, buf[:3]); err != nil {
		return err
	}

	rep := buf[1]
	if rep != 0 {
		return errors.New("SOCKS handshake failed")
	}

	remoteAddr, err := readAddr(remoteConn, buf)
	if err != nil {
		return err
	}

	resolvedRemoteAddr, err := net.ResolveUDPAddr("udp", remoteAddr.String())
	if err != nil {
		return errors.New("failed to resolve remote address")
	}

	go h.handleTCP(conn, remoteConn)

	remoteUDPConn, err := net.ListenPacket("udp", "")
	if err != nil {
		return err
	}

	var process string
	if h.sessionStater != nil {
		// Get name of the process.
		localHost, localPortStr, _ := net.SplitHostPort(conn.LocalAddr().String())
		localPortInt, _ := strconv.Atoi(localPortStr)
		process, err = lsof.GetCommandNameBySocket(conn.LocalAddr().Network(), localHost, uint16(localPortInt))
		if err != nil {
			process = "N/A"
		}

		sess := &stats.Session{
			ProcessName:   process,
			Network:       conn.LocalAddr().Network(),
			DialerAddr:    remoteConn.LocalAddr().String(),
			ClientAddr:    conn.LocalAddr().String(),
			TargetAddr:    targetAddr,
			UploadBytes:   0,
			DownloadBytes: 0,
			SessionStart:  time.Now(),
		}
		h.sessionStater.AddSession(conn, sess)

		remoteUDPConn = stats.NewSessionPacketConn(remoteUDPConn, sess)
	}

	h.remoteAddrMap.Store(conn, resolvedRemoteAddr)
	h.remoteConnMap.Store(conn, remoteConn)
	h.remotePacketConnMap.Store(conn, remoteUDPConn)

	go h.fetchUDPInput(conn, remoteUDPConn)

	log.Access(process, "proxy", "udp", conn.LocalAddr().String(), targetAddr)
	return nil
}

func (h *udpHandler) ReceiveTo(conn core.UDPConn, data []byte, addr *net.UDPAddr) error {
	var remoteAddr net.Addr
	var remoteUDPConn net.PacketConn

	if value, ok := h.remotePacketConnMap.Load(conn); ok {
		remoteUDPConn = value.(net.PacketConn)
	}

	if value, ok := h.remoteAddrMap.Load(conn); ok {
		remoteAddr = value.(net.Addr)
	}

	if remoteAddr == nil || remoteUDPConn == nil {
		h.Close(conn)
		return errors.New(fmt.Sprintf("proxy connection %v->%v does not exists", conn.LocalAddr(), addr))
	}

	var targetHost = addr.IP.String()
	if h.fakeDns != nil {
		if host, exist := h.fakeDns.IPToHost(addr.IP); exist {
			targetHost = host
		}
	}

	targetAddr := net.JoinHostPort(targetHost, strconv.Itoa(addr.Port))
	buf := bytes.Join([][]byte{{0, 0, 0}, ParseAddr(targetAddr), data[:]}, []byte{})
	if _, err := remoteUDPConn.WriteTo(buf, remoteAddr); err != nil {
		h.Close(conn)
		return errors.New(fmt.Sprintf("write remote failed: %v", err))
	}

	return nil
}

func (h *udpHandler) Close(conn core.UDPConn) {
	if remoteConn, ok := h.remoteConnMap.Load(conn); ok {
		remoteConn.(net.Conn).Close()
		h.remoteConnMap.Delete(conn)
	}

	if remoteUDPConn, ok := h.remotePacketConnMap.Load(conn); ok {
		remoteUDPConn.(net.PacketConn).Close()
		h.remotePacketConnMap.Delete(conn)
	}

	conn.Close()
	h.remoteAddrMap.Delete(conn)

	if h.sessionStater != nil {
		h.sessionStater.RemoveSession(conn)
	}
}
