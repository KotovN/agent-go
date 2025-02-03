package team

import (
	"context"
	"fmt"
	"sync"
	"time"

	"agentai/pkg/tasks"
)

// Vote represents a single agent's vote
type Vote struct {
	AgentID   string
	Decision  string // "APPROVE" or "REJECT"
	Rationale string
	Timestamp time.Time
}

// VotingSession manages voting for a specific task
type VotingSession struct {
	TaskID    string
	Config    VotingConfig
	Votes     []Vote
	Round     int
	StartTime time.Time
	mu        sync.RWMutex
}

// NewVotingSession creates a new voting session
func NewVotingSession(taskID string, config VotingConfig) *VotingSession {
	return &VotingSession{
		TaskID:    taskID,
		Config:    config,
		Votes:     make([]Vote, 0),
		Round:     1,
		StartTime: time.Now(),
	}
}

// CastVote adds a vote to the session
func (vs *VotingSession) CastVote(agentID, decision, rationale string) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	// Validate vote
	if decision != "APPROVE" && decision != "REJECT" {
		return fmt.Errorf("invalid vote decision: %s", decision)
	}

	// Check for duplicate votes
	for _, v := range vs.Votes {
		if v.AgentID == agentID {
			return fmt.Errorf("agent %s has already voted", agentID)
		}
	}

	vote := Vote{
		AgentID:   agentID,
		Decision:  decision,
		Rationale: rationale,
		Timestamp: time.Now(),
	}

	vs.Votes = append(vs.Votes, vote)
	return nil
}

// GetVotingResults calculates current voting results
func (vs *VotingSession) GetVotingResults() (bool, float64, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	if len(vs.Votes) == 0 {
		return false, 0, fmt.Errorf("no votes cast")
	}

	approveCount := 0
	for _, v := range vs.Votes {
		if v.Decision == "APPROVE" {
			approveCount++
		}
	}

	approvalPercentage := float64(approveCount) / float64(len(vs.Votes)) * 100

	// Check if consensus is reached
	consensusReached := false
	if vs.Config.RequireUnanimous {
		consensusReached = approvalPercentage == 100
	} else {
		consensusReached = approvalPercentage >= float64(vs.Config.ConsensusThreshold)
	}

	return consensusReached, approvalPercentage, nil
}

// HandleVotingRound manages a complete voting round
func (c *Team) HandleVotingRound(ctx context.Context, task *tasks.Task) (bool, error) {
	session := NewVotingSession(task.ID, DefaultVotingConfig)

	// Collect votes from all agents
	for _, agent := range c.Agents {
		voteCtx := fmt.Sprintf("Task: %s\nDescription: %s\nExpected Output: %s\n\nPlease vote %s",
			task.ID, task.Description, task.ExpectedOutput, session.Config.VoteFormat)

		decision, rationale, err := agent.GetVoteDecision(ctx, voteCtx)
		if err != nil {
			return false, fmt.Errorf("failed to get vote from agent %s: %w", agent.Name, err)
		}

		if err := session.CastVote(agent.Name, decision, rationale); err != nil {
			return false, fmt.Errorf("failed to cast vote: %w", err)
		}
	}

	// Calculate results
	consensusReached, percentage, err := session.GetVotingResults()
	if err != nil {
		return false, fmt.Errorf("failed to get voting results: %w", err)
	}

	// Store voting results in pipeline context
	c.pipeline.SetTaskContext(task.ID, fmt.Sprintf("voting_round_%d", session.Round), map[string]interface{}{
		"consensus_reached":   consensusReached,
		"approval_percentage": percentage,
		"votes":               session.Votes,
	})

	return consensusReached, nil
}
