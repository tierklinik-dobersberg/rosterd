package generator

import (
	"math/rand"

	"github.com/ccssmnn/hego"
)

type AnnealState struct {
	GeneratorState
}

func NewAnnealState(state GeneratorState) *AnnealState {
	return &AnnealState{GeneratorState: state}
}

func (s *AnnealState) Neighbor() hego.AnnealingState {
	if rand.Float64() < 0.5 {
		return NewAnnealState(*s.swapShift())
	}
	return NewAnnealState(*s.transferShift())
}

func (s *AnnealState) Energy() float64 {
	r := s.ToRoster()

	return -1000 + float64(s.getObjective(r))
}
