package awt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os/exec"
	"runtime"
	"time"
)

type Trajectory struct {
	Speed    int64  `json:"speed"`
	Duration uint64 `json:"duration"`
	Latency  uint64 `json:"latency"`
}

type BandwidthReport struct {
	CurrentBandwidth int64 `json:"currentBandwidth"`
}

func (r *BandwidthReport) Encode() (buf []byte, err error) {
	if buf, err = json.Marshal(r); err != nil {
		log.Printf("Error: %s\n", err)
		return
	}
	return
}

type BandwidthLimiter struct {
	CurrentBandwidth  int64
	CurrentTrajectory []Trajectory
	DoAbort           bool
	DefaultLatency    uint64
	NetworkInterfaces []net.Interface
}

func NewBandwidthLimiter() (l *BandwidthLimiter, err error) {
	var (
		iFaces []net.Interface
	)
	if runtime.GOOS != "linux" {
		err = ErrInvalidOS
		log.Printf("Error: %s\n", err)
		return
	}

	if iFaces, err = net.Interfaces(); err != nil {
		log.Printf("Error: %s\n", err)
		return
	}

	l = &BandwidthLimiter{
		CurrentBandwidth:  -1,
		DefaultLatency:    50,
		NetworkInterfaces: iFaces,
	}
	return
}

func (l *BandwidthLimiter) SetBandwidth(trajectory []Trajectory) {
	var (
		err error
	)

	if len(trajectory) <= 0 {
		log.Println("no trajectory was given")
		return
	}

	l.CurrentTrajectory = trajectory

	log.Println("limiting bandwidth...")

	for _, step := range l.CurrentTrajectory {
		var (
			bandwidthStr string = fmt.Sprintf("%dkbit", step.Speed)
			latencyStr   string
			latency      uint64
		)

		if step.Latency <= 0 {
			latency = l.DefaultLatency
		} else {
			latency = step.Latency
		}

		latencyStr = fmt.Sprintf("%dms", latency)

		l.CurrentBandwidth = step.Speed

		if step.Duration <= 0 {
			log.Printf("limiting to %s for eternity (or until changed)\n", bandwidthStr)
		} else {
			log.Printf("limiting to %s for the next %0.2fs\n", bandwidthStr, float64(step.Duration)/1_000)
		}

		if err = l.commandOnAllNetInterfaces(func(iface string) *exec.Cmd {
			return exec.Command("tc", "qdisc", "add", "dev", iface, "root", "tbf", "rate", bandwidthStr, "latency", latencyStr, "burst", "1540")
		}); err != nil {
			log.Printf("Error: %s\n", err)
			return
		}

		for dur := 1; dur <= int(step.Duration/200); dur++ {
			if !l.DoAbort {
				time.Sleep(200 * time.Millisecond)
			} else {
				l.DeleteBandwidth()
				l.DoAbort = false
				log.Println("aborted")
				return
			}
		}
	}
}

func (l *BandwidthLimiter) DeleteBandwidth() {
	var (
		err error
	)
	l.Abort()
	log.Println("deleting bandwidth...")

	if err = l.commandOnAllNetInterfaces(func(iface string) *exec.Cmd {
		return exec.Command("tc", "qdisc", "delete", "dev", iface, "root")
	}); err != nil {
		log.Printf("Error: %s\n", err)
		return
	}
}

func (l *BandwidthLimiter) Abort() {
	log.Println("aborting...")
	l.DoAbort = true
}

func (l *BandwidthLimiter) GetCurrentBandwidth() BandwidthReport {
	return BandwidthReport{
		CurrentBandwidth: l.CurrentBandwidth,
	}
}

func (l *BandwidthLimiter) commandOnAllNetInterfaces(tc func(iface string) *exec.Cmd) (err error) {
	for _, iface := range l.NetworkInterfaces {
		if iface.Name == "lo" {
			continue
		}

		var (
			cmd            *exec.Cmd
			stdout, stderr bytes.Buffer
		)

		cmd = tc(iface.Name)
		cmd.Stderr = &stderr
		cmd.Stdout = &stdout

		if err = cmd.Run(); err != nil {
			err = fmt.Errorf("%s: %s", err, stderr.String())
			log.Printf("Error: %s\n", err)
			return
		}
	}

	return
}
