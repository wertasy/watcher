package inotify

import (
	"fmt"
	"syscall"
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

	ch chan NetlinkMsg
}

type NetlinkMsg syscall.NetlinkMessage

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

	w := &NetlinkWatcher{fd: fd, sa: saddr, ch: make(chan NetlinkMsg)}

	go w.readLoop()

	return w, nil
}

func (l *NetlinkWatcher) readLoop() error {
	pkt := make([]byte, 4096)

	for {
		n, err := syscall.Read(l.fd, pkt)
		if err != nil {
			// return fmt.Errorf("failed to read: %s", err)
			continue
		}

		msgs, err := syscall.ParseNetlinkMessage(pkt[:n])
		if err != nil {
			// return fmt.Errorf("failed to parse: %s", err)
			continue
		}

		for _, msg := range msgs {
			l.ch <- NetlinkMsg(msg)
		}
	}
}

func (w *NetlinkWatcher) Wait() <-chan NetlinkMsg {
	return w.ch
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
