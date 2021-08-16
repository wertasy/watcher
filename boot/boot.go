package boot

import (
	"errors"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path"
	"strconv"

	"canhui.wang/factory/watcher"
)

type Ctrler interface {
	PowerOn()
}

type normalBootCtrler struct{}
type updateBootCtrler struct{}
type rollbackBootCtrler struct{}

func (*normalBootCtrler) PowerOn() {
	log.Println("Normal Power On")
}

func (*updateBootCtrler) PowerOn() {
	log.Println("Update Power On")
}

func (*rollbackBootCtrler) PowerOn() {
	log.Println("RollBack Power On")
}

func prePowerOn() {
	// 所有场景均要进行的准备工作
	waitBSPCreateSSDSymLink()
	waitBSPSetCtrlPanelIP()
}

func waitBSPCreateSSDSymLink() {
	const symLink = "/tmp/ssd"

	if IsExist(symLink) {
		log.Printf("symbolic link %s already existed", symLink)
		return
	}

	log.Printf("symbolic link %s does not exist, waiting for bsp to create", symLink)
	WaitCreate(symLink)
	log.Println(symLink, "created")
}

func IsExist(name string) bool {
	_, err := os.Stat(name)
	return !errors.Is(err, os.ErrNotExist)
}

func WaitCreate(name string) {
	w, err := watcher.NewInodeWatcher()
	if err != nil {
		panic(err)
	}

	if err = w.Add(path.Dir(name), watcher.IN_CREATE); err != nil {
		panic(err)
	}
	defer w.Close()

	for event := range w.Wait() {
		if event.IsCreate() && event.Name == path.Base(name) {
			break
		}
	}
}

func ListAllIPAddrs() ([]net.Addr, error) {
	ifces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var res []net.Addr
	for _, ifce := range ifces {
		addrs, err := ifce.Addrs()
		if err != nil {
			return nil, err
		}
		res = append(res, addrs...)
	}

	return res, nil
}

func GetCtrlPanelIP() (net.IP, error) {
	addrs, err := ListAllIPAddrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		if ipAddr, ok := addr.(*net.IPNet); ok && IsCtrlPanel(ipAddr.IP) {
			return ipAddr.IP, nil
		}
	}

	return nil, errors.New("no such ip addr")
}

func IsCtrlPanel(ip net.IP) bool {
	ip4 := ip.To4()
	return ip4 != nil && ip4[0] == 173
}

func waitBSPSetCtrlPanelIP() {
	if ip, err := GetCtrlPanelIP(); err == nil {
		log.Printf("control panel ip: %s already existed", ip)
		return
	}

	log.Printf("control panel ip does not exist, waiting for bsp to set")
	w, err := watcher.NewNetlinkWatcher()
	if err != nil {
		panic(err)
	}
	defer w.Close()

	var ip net.IP
	for msg := range w.Wait() {
		if m := msg.ParseAddrMsg(); IsCtrlPanel(m.Addr) {
			ip = m.Addr
			break
		}
	}
	log.Printf("contorl panel ip:%s already set", ip)

	SetMasterExclusiveIP(ip)
}

func SetMasterExclusiveIP(ip net.IP) {
	ip[2] = 95
	ip[3] = 16
	log.Println("set master node exclusive ip:", ip)
}

func PowerOn() {
	prePowerOn()
	LoadCtrler().PowerOn()
	postPowerOn()
}

func postPowerOn() {
	log.Println("check key image of protect area")
	log.Println("version compare")
}

// LoadCtrler 简单工厂
func LoadCtrler() (c Ctrler) {
	flag, err := readBootFlagFromFile()
	if err != nil {
		panic(err)
	}

	switch flag {
	case 0:
		c = &normalBootCtrler{}
	case 2:
		c = &updateBootCtrler{}
	case 3:
		c = &rollbackBootCtrler{}
	default:
		panic("")
	}

	return c
}

func readBootFlagFromFile() (flag int, err error) {
	bytes, err := ioutil.ReadFile("boot.flag")
	if err != nil {
		return
	}

	flag, err = strconv.Atoi(string(bytes))
	return
}
