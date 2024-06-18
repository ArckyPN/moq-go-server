/*
Copyright (c) Meta Platforms, Inc. and affiliates.
This source code is licensed under the MIT license found in the
LICENSE file in the root directory of this source tree.
*/

package moqmessageobjects

import (
	"errors"
	"facebookexperimental/moq-go-server/awt"
	"facebookexperimental/moq-go-server/moqobject"
	"fmt"
	"slices"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"
)

// File Definition of files
type MoqMessageObjects struct {
	// map[cacheKey]map[bitrate]MoqObject
	dataMap map[string]map[uint64]*moqobject.MoqObject

	// FilesLock Lock used to write / read files
	mapLock *sync.RWMutex

	// Housekeeping thread channel
	cleanUpChannel chan bool
}

// New Creates a new mem files map
func New(housekeepingPeriodMs uint64) *MoqMessageObjects {
	moqtObjs := MoqMessageObjects{dataMap: map[string]map[uint64]*moqobject.MoqObject{}, mapLock: new(sync.RWMutex), cleanUpChannel: make(chan bool)}

	if housekeepingPeriodMs > 0 {
		moqtObjs.startCleanUp(housekeepingPeriodMs)
	}

	return &moqtObjs
}

func (moqtObjs *MoqMessageObjects) Create(cacheKey string, objHeader moqobject.MoqObjectHeader, defObjExpirationS uint64) (moqObjs map[uint64]*moqobject.MoqObject, err error) {
	moqtObjs.mapLock.Lock()
	defer moqtObjs.mapLock.Unlock()

	_, found := moqtObjs.dataMap[cacheKey]
	if found /* && !foundObj.GetEof() */ {
		err = errors.New("We can NOT override on open object")
		return
	}

	// create a new Object for every Quality (bitrate)
	moqObjs = map[uint64]*moqobject.MoqObject{}
	for _, quality := range awt.EncoderSettings {
		moqObj := moqobject.New(objHeader, defObjExpirationS)
		moqObjs[quality.Bitrate] = moqObj
	}
	moqtObjs.dataMap[cacheKey] = moqObjs

	return
}

// gets the best fitting MoqObject by bitrate
func (moqtObjs *MoqMessageObjects) get(cacheKey string, etp uint64) (moqObjRet *moqobject.MoqObject, found bool) {
	moqtObjs.mapLock.RLock()
	defer moqtObjs.mapLock.RUnlock()

	var (
		bestFitBitrate uint64
		bitrates       []uint64
		objs           map[uint64]*moqobject.MoqObject
	)

	if objs, found = moqtObjs.dataMap[cacheKey]; !found {
		return
	}
	bitrates = maps.Keys(objs)
	slices.Sort(bitrates)

	for _, br := range bitrates {
		if br <= etp {
			bestFitBitrate = br
		}
		if br > etp {
			break
		}
	}

	// no fitting bitrate found, use the lowest
	if !slices.Contains(bitrates, bestFitBitrate) {
		return objs[bitrates[0]], true
	}

	moqObjRet = objs[bestFitBitrate]

	return
}

func (moqtObjs *MoqMessageObjects) Get(cacheKey string, bitrate uint64) (moqObjRet *moqobject.MoqObject, found bool) {
	moqObjRet, found = moqtObjs.get(cacheKey, bitrate)

	return
}

func (moqtObjs *MoqMessageObjects) Stop() {
	moqtObjs.stopCleanUp()
}

// Housekeeping

func (moqtObjs *MoqMessageObjects) startCleanUp(periodMs uint64) {
	go moqtObjs.runCleanupEvery(periodMs, moqtObjs.cleanUpChannel)

	log.Info("Started clean up thread")
}

func (moqtObjs *MoqMessageObjects) stopCleanUp() {
	// Send finish signal
	moqtObjs.cleanUpChannel <- true

	// Wait to finish
	<-moqtObjs.cleanUpChannel

	log.Info("Stopped clean up thread")
}

func (moqtObjs *MoqMessageObjects) runCleanupEvery(periodMs uint64, cleanUpChannelBidi chan bool) {
	timeCh := time.NewTicker(time.Millisecond * time.Duration(periodMs))
	exit := false

	for !exit {
		select {
		// Wait for the next tick
		case tm := <-timeCh.C:
			moqtObjs.cacheCleanUp(tm)

		case <-cleanUpChannelBidi:
			exit = true
		}
	}
	// Indicates finished
	cleanUpChannelBidi <- true

	log.Info("Exited clean up thread")
}

func (moqtObjs *MoqMessageObjects) cacheCleanUp(now time.Time) {
	objectsToDel := map[string]*moqobject.MoqObject{}

	// TODO: This is a brute force approach, optimization recommended

	moqtObjs.mapLock.Lock()
	defer moqtObjs.mapLock.Unlock()

	numStartElements := len(moqtObjs.dataMap)

	// Check for expired files
	for key, objs := range moqtObjs.dataMap {
		for _, obj := range objs {
			if obj.MaxAgeS >= 0 && obj.GetEof() {
				if obj.ReceivedAt.Add(time.Second * time.Duration(obj.MaxAgeS)).Before(now) {
					objectsToDel[key] = obj
				}
			}
		}
	}
	// Delete expired files
	for keyToDel := range objectsToDel {
		// Delete from array
		delete(moqtObjs.dataMap, keyToDel)
		log.Info("CLEANUP MOQ object expired, deleted: ", keyToDel)
	}

	numEndElements := len(moqtObjs.dataMap)

	log.Info(fmt.Sprintf("Finished cleanup MOQ objects round expired. Elements at start: %d, elements at end: %d", numStartElements, numEndElements))
}
