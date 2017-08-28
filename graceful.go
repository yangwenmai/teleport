// Copyright 2015-2017 HenryLee. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tp

import (
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/henrylee2cn/goutil/errors"
	"github.com/henrylee2cn/goutil/graceful"
	"github.com/henrylee2cn/goutil/graceful/inherit_net"
)

var peers = struct {
	list map[*Peer]struct{}
	rwmu sync.RWMutex
}{
	list: make(map[*Peer]struct{}),
}

func addPeer(p *Peer) {
	peers.rwmu.Lock()
	peers.list[p] = struct{}{}
	peers.rwmu.Unlock()
}

func deletePeer(p *Peer) {
	peers.rwmu.Lock()
	delete(peers.list, p)
	peers.rwmu.Unlock()
}

func shutdown() error {
	peers.rwmu.Lock()
	defer peers.rwmu.Unlock()
	var (
		count int
		errCh = make(chan error, len(peers.list))
	)
	for p := range peers.list {
		count++
		go func(peer *Peer) {
			errCh <- peer.Close()
		}(p)
	}
	var err error
	for i := 0; i < count; i++ {
		err = errors.Merge(err, <-errCh)
	}
	close(errCh)
	return err
}

func init() {
	SetShutdown(5*time.Second, nil, nil)
}

// GraceSignal open graceful shutdown or reboot signal.
func GraceSignal() {
	graceful.GraceSignal()
}

// FirstSweep is first executed.
// BeforeExiting is executed before process exiting.
// Usage: share github.com/henrylee2cn/goutil/graceful with other project.
var FirstSweep, BeforeExiting func() error

// SetShutdown sets the function which is called after the process shutdown,
// and the time-out period for the process shutdown.
// If 0<=timeout<5s, automatically use 'MinShutdownTimeout'(5s).
// If timeout<0, indefinite period.
// 'firstSweep' is first executed.
// 'beforeExiting' is executed before process exiting.
func SetShutdown(timeout time.Duration, firstSweep, beforeExiting func() error) {
	if firstSweep == nil {
		firstSweep = func() error { return nil }
	}
	if beforeExiting == nil {
		beforeExiting = func() error { return nil }
	}
	FirstSweep = func() error {
		return errors.Merge(firstSweep(), inherit_net.SetInherited())
	}
	BeforeExiting = func() error {
		return errors.Merge(shutdown(), beforeExiting())
	}
	graceful.SetShutdown(timeout, FirstSweep, BeforeExiting)
}

// Shutdown closes all the frame process gracefully.
// Parameter timeout is used to reset time-out period for the process shutdown.
func Shutdown(timeout ...time.Duration) {
	graceful.Shutdown(timeout...)
}

// Reboot all the frame process gracefully.
// Notes: Windows system are not supported!
func Reboot(timeout ...time.Duration) {
	graceful.Reboot(timeout...)
}

func listen(laddr string, tlsConfig *tls.Config) (net.Listener, error) {
	lis, err := inherit_net.Listen("tcp", laddr)
	if err != nil {
		return nil, err
	}
	if tlsConfig != nil {
		if len(tlsConfig.Certificates) == 0 && tlsConfig.GetCertificate == nil {
			return nil, errors.New("tls: neither Certificates nor GetCertificate set in Config")
		}
		lis = tls.NewListener(lis, tlsConfig)
	}
	return lis, nil
}
