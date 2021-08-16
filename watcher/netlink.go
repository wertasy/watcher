package watcher

import (
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

const (
	RTM_NEWADDR = syscall.RTM_NEWADDR
	RTM_DELADDR = syscall.RTM_DELADDR
	// golang syscall.RTNLGRP_IPV4_IFADDR=0x5,
	// but in linux kernel #define RTMGRP_IPV4_IFADDR 0x10
	// see also: https://github.com/golang/go/issues/15080
	RTNLGRP_IPV4_IFADDR = 0x10
)

type NetlinkWatcher struct {
	fd int
	sa *syscall.SockaddrNetlink

	ch chan *NetlinkMsg
}

// NetlinkMessage represents a netlink message.
// type NetlinkMessage struct {
// 	Header NlMsghdr
// 	Data   []byte
// }
type NetlinkMsg syscall.NetlinkMessage

type AddrMsg struct {
	IfAddrmsg syscall.IfAddrmsg
	RtAttr    syscall.RtAttr
	Addr      net.IP
	Mask      net.IPMask
}

func (m *NetlinkMsg) ParseAddrMsg() *AddrMsg {
	msg := (*syscall.IfAddrmsg)(unsafe.Pointer(&m.Data[0]))
	attr := (*syscall.RtAttr)(unsafe.Pointer(&m.Data[syscall.SizeofIfAddrmsg]))
	begin := syscall.SizeofIfAddrmsg + syscall.SizeofRtAttr
	end := syscall.SizeofIfAddrmsg + attr.Len
	addr := m.Data[begin:end]
	return &AddrMsg{
		IfAddrmsg: *msg,
		RtAttr:    *attr,
		Addr:      addr,
		Mask:      net.CIDRMask(int(msg.Prefixlen), 8*len(addr)),
	}
}

func NewNetlinkWatcher() (*NetlinkWatcher, error) {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_DGRAM, syscall.NETLINK_ROUTE)
	if err != nil {
		return nil, fmt.Errorf("failed to create socket: %w", err)
	}

	saddr := &syscall.SockaddrNetlink{
		Family: syscall.AF_NETLINK,
		Pid:    0,
		Groups: syscall.RTNLGRP_LINK | RTNLGRP_IPV4_IFADDR | syscall.RTNLGRP_IPV6_IFADDR,
	}

	err = syscall.Bind(fd, saddr)
	if err != nil {
		return nil, fmt.Errorf("failed to bind socket: %w", err)
	}

	w := &NetlinkWatcher{fd: fd, sa: saddr, ch: make(chan *NetlinkMsg)}

	go w.readLoop()

	return w, nil
}

func (w *NetlinkWatcher) readLoop() error {
	buff := make([]byte, 4096)

	for {
		n, err := syscall.Read(w.fd, buff)
		if err != nil {
			// return fmt.Errorf("failed to read: %s", err)
			continue
		}

		msgs, err := syscall.ParseNetlinkMessage(buff[:n])
		if err != nil {
			// return fmt.Errorf("failed to parse: %s", err)
			continue
		}

		for _, msg := range msgs {
			w.ch <- (*NetlinkMsg)(unsafe.Pointer(&msg))
		}
	}
}

func (w *NetlinkWatcher) Wait() <-chan *NetlinkMsg {
	return w.ch
}

func (w *NetlinkWatcher) Close() {
	syscall.Close(w.fd)
}

func (msg *NetlinkMsg) IsNewAddr() bool {
	return msg.Header.Type == RTM_NEWADDR
}

func (msg *NetlinkMsg) IsDelAddr() bool {
	return msg.Header.Type == RTM_DELADDR
}

func IsRelevant(msg *syscall.IfAddrmsg) bool {
	return msg.Scope == syscall.RT_SCOPE_UNIVERSE || msg.Scope == syscall.RT_SCOPE_SITE
}
