// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package tendermint

import (
	"context"
	"github.com/clearmatics/autonity/core/types"
	"time"

	"github.com/clearmatics/autonity/common"
	"github.com/clearmatics/autonity/event"
)

// Backend provides application specific functions for Istanbul core
type Backend interface {
	// Address returns the owner's address
	Address() common.Address

	// Validators returns the validator set
	Validators(number uint64) ValidatorSet

	// EventMux returns the event mux in backend
	EventMux() *event.TypeMuxSilent

	// Broadcast sends a message to all validators (include self)
	Broadcast(ctx context.Context, valSet ValidatorSet, payload []byte) error

	// Gossip sends a message to all validators (exclude self)
	Gossip(ctx context.Context, valSet ValidatorSet, payload []byte)

	// Commit delivers an approved proposal to backend.
	// The delivered proposal will be put into blockchain.
	Commit(proposalBlock types.Block, seals [][]byte) error

	// VerifyProposal verifies the proposal. If a consensus.ErrFutureBlock error is returned,
	// the time difference of the proposal and current time is also returned.
	VerifyProposal(types.Block) (time.Duration, error)

	// Sign signs input data with the backend's private key
	Sign([]byte) ([]byte, error)

	// CheckSignature verifies the signature by checking if it's signed by
	// the given validator
	CheckSignature(data []byte, addr common.Address, sig []byte) error

	// LastCommittedProposal retrieves latest committed proposal and the address of proposer
	LastCommittedProposal() (*types.Block, common.Address)

	// GetProposer returns the proposer of the given block height
	GetProposer(number uint64) common.Address

	// HasBadBlock returns whether the block with the hash is a bad block
	HasBadProposal(hash common.Hash) bool

	// Setter for proposed block hash
	SetProposedBlockHash(hash common.Hash)
}