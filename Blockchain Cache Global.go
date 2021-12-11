/*
File Name:  Blockchain Cache Global.go
Copyright:  2021 Peernet s.r.o.
Author:     Peter Kleissner
*/

package core

import (
	"github.com/PeernetOfficial/core/blockchain"
	"github.com/PeernetOfficial/core/protocol"
	"github.com/enfipy/locker"
)

// The blockchain cache stores blockchains.
type BlockchainCache struct {
	BlockchainDirectory string // The directory for storing blockchains in a key-value store.
	MaxBlockSize        uint64 // Max block size to accept.
	MaxBlockCount       uint64 // Max block count to cache per peer.
	LimitTotalRecords   uint64 // Max count of blocks and header in total to keep across all blockchains. 0 = unlimited. Max Records * Max Block Size = Size Limit.
	ReadOnly            bool   // Whether the cache is read only.

	store    *blockchain.MultiStore
	peerLock *locker.Locker
}

func initBlockchainCache(BlockchainDirectory string, MaxBlockSize, MaxBlockCount, LimitTotalRecords uint64) (cache *BlockchainCache) {
	if BlockchainDirectory == "" {
		return nil
	}

	var err error

	cache = &BlockchainCache{
		BlockchainDirectory: BlockchainDirectory,
		MaxBlockSize:        MaxBlockSize,
		MaxBlockCount:       MaxBlockCount,
		LimitTotalRecords:   LimitTotalRecords,
	}

	cache.store, err = blockchain.InitMultiStore(BlockchainDirectory)
	if err != nil {
		Filters.LogError("initBlockchainCache", "initializing database '%s': %s", BlockchainDirectory, err.Error())
		return nil
	}

	cache.peerLock = locker.Initialize()

	if LimitTotalRecords > 0 && cache.store.Database.Count() >= LimitTotalRecords {
		cache.ReadOnly = true
	}

	return cache
}

// SeenBlockchainVersion shall be called with information about another peer's blockchain.
// If the reported version number is newer, all existing blocks are immediately deleted.
func (cache *BlockchainCache) SeenBlockchainVersion(peer *PeerInfo) {
	cache.peerLock.Lock(string(peer.PublicKey.SerializeCompressed()))
	defer cache.peerLock.Unlock(string(peer.PublicKey.SerializeCompressed()))

	// intermediate function to download and process blocks
	downloadAndProcessBlocks := func(peer *PeerInfo, header *blockchain.MultiBlockchainHeader, offset, limit uint64) {
		oldCount := len(header.ListBlocks)

		if limit > cache.MaxBlockCount {
			limit = cache.MaxBlockCount
		}

		peer.BlockDownload(peer.PublicKey, cache.MaxBlockCount, cache.MaxBlockSize, []protocol.BlockRange{{Offset: offset, Limit: limit}}, func(data []byte, targetBlock protocol.BlockRange, blockSize uint64, availability uint8) {
			if availability != protocol.GetBlockStatusAvailable {
				return
			}
			cache.store.WriteBlock(peer.PublicKey, peer.BlockchainVersion, targetBlock.Offset, data)
			header.ListBlocks = append(header.ListBlocks, targetBlock.Offset)
		})

		// only update the blockchain header if it changed
		if oldCount != len(header.ListBlocks) {
			cache.store.WriteBlockchainHeader(peer.PublicKey, header)
		}
	}

	// get the old header
	header, status, err := cache.store.AssessBlockchainHeader(peer.PublicKey, peer.BlockchainVersion, uint64(peer.BlockchainHeight))
	if err != nil {
		return
	}

	switch status {
	case blockchain.MultiStatusEqual:
		return

	case blockchain.MultiStatusInvalidRemote:
		cache.store.DeleteBlockchain(peer.PublicKey, header)

	case blockchain.MultiStatusHeaderNA:
		if header, err = cache.store.NewBlockchainHeader(peer.PublicKey, peer.BlockchainVersion, uint64(peer.BlockchainHeight)); err != nil {
			return
		}

		downloadAndProcessBlocks(peer, header, 0, uint64(peer.BlockchainHeight))

	case blockchain.MultiStatusNewVersion:
		// delete existing data first, then create it new
		cache.store.DeleteBlockchain(peer.PublicKey, header)

		if header, err = cache.store.NewBlockchainHeader(peer.PublicKey, peer.BlockchainVersion, uint64(peer.BlockchainHeight)); err != nil {
			return
		}

		downloadAndProcessBlocks(peer, header, 0, uint64(peer.BlockchainHeight))

	case blockchain.MultiStatusNewBlocks:
		offset := header.Height
		limit := uint64(peer.BlockchainHeight) - header.Height

		header.Height = uint64(peer.BlockchainHeight)

		downloadAndProcessBlocks(peer, header, offset, limit)

	}

	if cache.LimitTotalRecords > 0 {
		cache.ReadOnly = cache.store.Database.Count() >= cache.LimitTotalRecords
	}
}

// remoteBlockchainUpdate shall be called to indicate a potential update of the remotes blockchain.
// It will use the blockchain version and height to update the data lake as appropriate.
// This function is called in the Go routine of the packet worker and therefore must not stall.
func (peer *PeerInfo) remoteBlockchainUpdate() {
	if currentBackend.GlobalBlockchainCache == nil || currentBackend.GlobalBlockchainCache.ReadOnly || peer.BlockchainVersion == 0 && peer.BlockchainHeight == 0 {
		return
	}

	// TODO: This entire function should be instead a non-blocking message via a buffer channel.
	go currentBackend.GlobalBlockchainCache.SeenBlockchainVersion(peer)
}
