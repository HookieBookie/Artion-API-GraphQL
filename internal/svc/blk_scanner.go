// Package svc implements monitoring and scanning services of the API server.
package svc

import (
	"artion-api-graphql/internal/repository"
	eth "github.com/ethereum/go-ethereum/core/types"
	"time"
)

const (
	// blkIsScanning represents the state of active block scanning
	blkIsScanning = iota

	// blkIsIdling represents the state of passive head checks
	blkIsIdling

	// defStartingBlockNumber represents the first block we will scan from
	// if the previous state is unknown.
	defStartingBlockNumber = 16000000

	// blkScannerHysteresis represent the number of blocks we let slide
	// until we switch back to active scan state.
	blkScannerHysteresis = 10
)

// blkScanner represents a scanner of historical data from the blockchain.
type blkScanner struct {
	// mgr represents the Manager instance
	mgr *Manager

	// sigStop represents the signal for closing the router
	sigStop chan bool

	// outBlocks represents a channel fed with historical
	// block headers being scanned.
	outBlocks chan *eth.Header

	// inObservedBlocks is a channel receiving IDs of observed blocks
	// we track the observed heads to recognize if we need to switch back to scan from idle
	inObservedBlocks chan uint64

	// outStateChange represents the channel being fed
	// with internal state change of the scanner.
	outStateChange chan int

	// state represents the current state of the scanner
	// it's scanning by default and turns idle later, if not needed
	state int

	// current represents the ID of the currently processed block
	current uint64

	// target represents the ID we need to reach
	target uint64
}

// newBlkScanner creates a new instance of the block scanner service.
func newBlkScanner(mgr *Manager) *blkScanner {
	return &blkScanner{
		mgr:            mgr,
		sigStop:        make(chan bool, 1),
		outBlocks:      make(chan *eth.Header, outBlockQueueCapacity),
		outStateChange: make(chan int),
	}
}

// name provides the name of the service.
func (bs *blkScanner) name() string {
	return "block scanner"
}

// init initializes the block scanner and registers it with the manager.
func (bs *blkScanner) init() {
	// get last known block
	bs.current, bs.target = bs.start(), bs.top()
	bs.mgr.add(bs)
}

// top provides the current end target for the scanner.
func (bs *blkScanner) top() uint64 {
	cur, err := repository.R().CurrentHead()
	if err != nil {
		log.Criticalf("can not pull the latest head number; %s", err.Error())
		return 0
	}
	return cur
}

// start provides the starting point for the scanner
func (bs *blkScanner) start() uint64 {
	lnb, err := repository.R().LastSeenBlockNumber()
	if err != nil {
		log.Criticalf("can not pull the previous state; %s", err.Error())
		return 0
	}

	// if the state is unknown, use default starting block number
	// we don't need to start scanning from the absolute start of the chain
	if lnb == 0 {
		return defStartingBlockNumber
	}
	return lnb
}

// run pulls block headers from multiple sources and routes based on the API server state.
func (bs *blkScanner) run() {
	// make tickers
	tgTick := time.NewTicker(2 * time.Second)
	logTick := time.NewTicker(10 * time.Second)

	defer func() {
		tgTick.Stop()
		logTick.Stop()
		bs.mgr.closed(bs)
	}()

	for {
		// make sure to check for terminate; but do not stay in
		select {
		case <-bs.sigStop:
			return
		case <-tgTick.C:
			bs.target = bs.top()
		case <-logTick.C:
			log.Infof("block scanner at #%d of #%d", bs.current, bs.target)
		case bid := <-bs.inObservedBlocks:
			if bs.state == blkIsIdling && bid > bs.current {
				bs.current = bid
			}
		default:
		}

		bs.scanNext()
		bs.checkTarget()
		bs.checkIdle()
	}
}

// scanNext tries to advance the scanner to the next block, if possible
func (bs *blkScanner) scanNext() {
	// if we are scanning and below target; get next one
	if bs.state == blkIsScanning && bs.current < bs.target {
		hdr, err := repository.R().PullHeader(bs.current)
		if err != nil {
			log.Errorf("block header #%s not available; %s", bs.current, err.Error())
			select {
			case <-bs.sigStop:
				bs.sigStop <- true
			case <-time.After(5 * time.Second):
			}
			return
		}

		// send the block to the router; make sure not to miss stop signal
		select {
		case bs.outBlocks <- hdr:
			bs.current += 1
		case <-bs.sigStop:
			bs.sigStop <- true
		}
	}
}

// checkTarget checks if the scanner reached designated target head.
func (bs *blkScanner) checkTarget() {
	// reached target? make sure we are on target; switch state if so
	if bs.state == blkIsScanning && bs.current > bs.target {
		bs.target = bs.top()
		diff := bs.target - bs.current
		if diff >= 0 && diff < blkScannerHysteresis {
			bs.state = blkIsIdling
			log.Noticef("scanner reached head; idling")

			select {
			case bs.outStateChange <- bs.state:
			default:
			}
		}
	}
}

// checkIdle checks if the idle state should be switched back to active scan.
func (bs *blkScanner) checkIdle() {
	if bs.state != blkIsIdling {
		return
	}

	diff := bs.target - bs.current
	if diff > blkScannerHysteresis {
		bs.state = blkIsScanning
		log.Noticef("scanner lost head; re-scan started")

		select {
		case bs.outStateChange <- bs.state:
		default:
		}
	}
}

// close signals the block observer to terminate
func (bs *blkScanner) close() {
	bs.sigStop <- true
}
