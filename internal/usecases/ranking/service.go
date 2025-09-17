package ranking

import (
	"github.com/vfg2006/traffic-manager-api/infrastructure/repository"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
)

type RankingService interface {
	GetStoreRanking() (*domain.StoreRankingResponse, error)
}

type StoreRankingService struct {
	StoreRankingRepository repository.StoreRankingRepository
}

func NewStoreRankingService(storeRankingRepository repository.StoreRankingRepository) RankingService {
	return &StoreRankingService{
		StoreRankingRepository: storeRankingRepository,
	}
}

func (s *StoreRankingService) GetStoreRanking() (*domain.StoreRankingResponse, error) {
	ranking, err := s.StoreRankingRepository.GetStoreRanking()
	if err != nil {
		return nil, err
	}
	return ranking, nil
}
