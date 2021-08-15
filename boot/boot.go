package boot

import (
	"errors"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strconv"

	"canhui.wang/factory/inotify"
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
	watcher, err := inotify.NewWatcher()
	if err != nil {
		panic(err)
	}

	if err = watcher.Add(path.Dir(name), inotify.IN_CREATE); err != nil {
		panic(err)
	}
	defer watcher.Close()

	for {
		if event := <-watcher.Wait(); event.IsCreate() && event.Name == path.Base(name) {
			break
		}
	}
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
