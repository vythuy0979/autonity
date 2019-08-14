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

package core

import (
	"context"
	"math/big"

	"github.com/clearmatics/autonity/common"
	"github.com/clearmatics/autonity/consensus/tendermint"
)

func (c *core) sendPrevote(ctx context.Context, isNil bool) {
	logger := c.logger.New("step", c.currentRoundState.Step())

	var prevote = tendermint.Vote{
		Round:  big.NewInt(c.currentRoundState.Round().Int64()),
		Height: big.NewInt(c.currentRoundState.Height().Int64()),
	}

	if isNil {
		prevote.ProposedBlockHash = common.Hash{}
	} else {
		if h := c.currentRoundState.GetCurrentProposalHash(); h == (common.Hash{}) {
			c.logger.Error("sendPrecommit Proposal is empty! It should not be empty!")
			return
		}
		prevote.ProposedBlockHash = c.currentRoundState.GetCurrentProposalHash()
	}

	encodedVote, err := Encode(&prevote)
	if err != nil {
		logger.Error("Failed to encode", "subject", prevote)
		return
	}

	c.logPrevoteMessageEvent("MessageEvent(Prevote): Sent", prevote, c.address.String(), "broadcast")

	c.sentPrevote = true
	c.broadcast(ctx, &message{
		Code: msgPrevote,
		Msg:  encodedVote,
	})
}

func (c *core) handlePrevote(ctx context.Context, msg *message) error {
	var preVote tendermint.Vote
	err := msg.Decode(&preVote)
	if err != nil {
		return errFailedDecodePrevote
	}

	if err = c.checkMessage(preVote.Round, preVote.Height); err != nil {
		// Store old round prevote messages for future rounds since it is required for validRound
		if err == errOldRoundMessage {
			// The roundstate must exist as every roundstate is added to c.currentHeightRoundsState at startRound
			// And we only process old rounds while future rounds messages are pushed on to the backlog
			oldRoundState := c.currentHeightOldRoundsStates[preVote.Round.Int64()]
			c.acceptVote(&oldRoundState, prevote, preVote.ProposedBlockHash, *msg)
		}
		return err
	}

	// After checking the message we know it is from the same height and round, so we should store it even if
	// c.currentRoundState.Step() < prevote. The propose timeout which is started at the beginning of the round
	// will update the step to at least prevote and when it handle its on preVote(nil), then it will also have
	// votes from other nodes.
	prevoteHash := preVote.ProposedBlockHash
	c.acceptVote(c.currentRoundState, prevote, prevoteHash, *msg)

	c.logPrevoteMessageEvent("MessageEvent(Prevote): Received", preVote, msg.Address.String(), c.address.String())

	// Now we can add the preVote to our current round state
	if c.currentRoundState.Step() >= prevote {
		curProposalHash := c.currentRoundState.GetCurrentProposalHash()
		curR := c.currentRoundState.Round().Int64()
		curH := c.currentRoundState.Height().Int64()

		// Line 36 in Algorithm 1 of The latest gossip on BFT consensus
		if curProposalHash != (common.Hash{}) && c.quorum(c.currentRoundState.Prevotes.VotesSize(curProposalHash)) && !c.setValidRoundAndValue {
			// this piece of code should only run once
			if err := c.prevoteTimeout.stopTimer(); err != nil {
				return err
			}
			c.logger.Debug("Stopped Scheduled Prevote Timeout")

			if c.currentRoundState.Step() == prevote {
				c.lockedValue = c.currentRoundState.Proposal().ProposalBlock
				c.lockedRound = big.NewInt(curR)
				c.sendPrecommit(ctx, false)
				c.setStep(precommit)
			}
			c.validValue = c.currentRoundState.Proposal().ProposalBlock
			c.validRound = big.NewInt(curR)
			c.setValidRoundAndValue = true
			// Line 44 in Algorithm 1 of The latest gossip on BFT consensus
		} else if c.currentRoundState.Step() == prevote && c.quorum(c.currentRoundState.Prevotes.NilVotesSize()) {
			if err := c.prevoteTimeout.stopTimer(); err != nil {
				return err
			}
			c.logger.Debug("Stopped Scheduled Prevote Timeout")

			c.sendPrecommit(ctx, true)
			c.setStep(precommit)

			// Line 34 in Algorithm 1 of The latest gossip on BFT consensus
		} else if c.currentRoundState.Step() == prevote && !c.prevoteTimeout.timerStarted() && !c.sentPrecommit && c.quorum(c.currentRoundState.Prevotes.TotalSize()) {
			timeoutDuration := timeoutPrevote(curR)
			c.prevoteTimeout.scheduleTimeout(timeoutDuration, curR, curH, c.onTimeoutPrevote)
			c.logger.Debug("Scheduled Prevote Timeout", "Timeout Duration", timeoutDuration)
		}
	}

	return nil
}

func (c *core) logPrevoteMessageEvent(message string, prevote tendermint.Vote, from, to string) {
	currentProposalHash := c.currentRoundState.GetCurrentProposalHash()
	c.logger.Debug(message,
		"from", from,
		"to", to,
		"currentHeight", c.currentRoundState.Height(),
		"msgHeight", prevote.Height,
		"currentRound", c.currentRoundState.Round(),
		"msgRound", prevote.Round,
		"currentStep", c.currentRoundState.Step(),
		"isProposer", c.isProposer(),
		"currentProposer", c.valSet.GetProposer(),
		"isNilMsg", prevote.ProposedBlockHash == common.Hash{},
		"hash", prevote.ProposedBlockHash,
		"type", "Prevote",
		"totalVotes", c.currentRoundState.Prevotes.TotalSize(),
		"totalNilVotes", c.currentRoundState.Prevotes.NilVotesSize(),
		"quorumReject", c.quorum(c.currentRoundState.Prevotes.NilVotesSize()),
		"totalNonNilVotes", c.currentRoundState.Prevotes.VotesSize(currentProposalHash),
		"quorumAccept", c.quorum(c.currentRoundState.Prevotes.VotesSize(currentProposalHash)),
	)
}