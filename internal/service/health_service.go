package service

import "fundtracking/internal/repository"

type VersionPayload struct {
	Service    string  `json:"service"`
	Version    string  `json:"version"`
	Commit     *string `json:"commit"`
	CommitFull *string `json:"commit_full"`
	BuiltAt    *string `json:"built_at"`
	ServerTime int64   `json:"server_time"`
}
type HealthService struct {
	runtimeRepo *repository.RuntimeRepository
}

func NewHealthService(runtimeRepo *repository.RuntimeRepository) *HealthService {
	return &HealthService{
		runtimeRepo: runtimeRepo,
	}
}
func (s *HealthService) GetVersion() VersionPayload {
	commitFull := s.runtimeRepo.CommitFull()
	var commit *string
	if commitFull != nil {
		short := *commitFull
		if len(short) > 7 {
			short = short[:7]
		}
		commit = &short
	}
	return VersionPayload{
		Service:    "fund-tracking-api",
		Version:    s.runtimeRepo.Version(),
		Commit:     commit,
		CommitFull: commitFull,
		BuiltAt:    s.runtimeRepo.BuiltAt(),
		ServerTime: s.runtimeRepo.ServerTime(),
	}
}
