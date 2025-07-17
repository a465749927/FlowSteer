package proxy

import (
	"encoding/binary"
	"net"
	"strconv"
	"syscall"
	"unsafe"
)

const soOriginalDst = 80

// originalDst returns the original destination address for a NAT redirected
// connection using SO_ORIGINAL_DST.
func originalDst(conn *net.TCPConn) (string, error) {
	rc, err := conn.SyscallConn()
	if err != nil {
		return "", err
	}
	var addr syscall.RawSockaddrInet4
	var l = uint32(syscall.SizeofSockaddrInet4)
	var serr error
	if err := rc.Control(func(fd uintptr) {
		_, _, e1 := syscall.Syscall6(syscall.SYS_GETSOCKOPT, fd,
			uintptr(syscall.SOL_IP), uintptr(soOriginalDst),
			uintptr(unsafe.Pointer(&addr)), uintptr(unsafe.Pointer(&l)), 0)
		if e1 != 0 {
			serr = e1
		}
	}); err != nil {
		return "", err
	}
	if serr != nil {
		return "", serr
	}
	ip := net.IPv4(addr.Addr[0], addr.Addr[1], addr.Addr[2], addr.Addr[3])
	b := *(*[2]byte)(unsafe.Pointer(&addr.Port))
	port := int(binary.BigEndian.Uint16(b[:]))
	return net.JoinHostPort(ip.String(), strconv.Itoa(port)), nil
}
