package meta

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	metadomain "github.com/vfg2006/traffic-manager-api/infrastructure/integrator/meta/domain"
	"github.com/vfg2006/traffic-manager-api/infrastructure/integrator/meta/metaclient"
	"github.com/vfg2006/traffic-manager-api/internal/config"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
	"github.com/vfg2006/traffic-manager-api/pkg/utils"
)

type MetaIntegrator struct {
	cfg    *config.Config
	Client metaclient.Client
}

func New(cfg *config.Config, client metaclient.Client) *MetaIntegrator {
	return &MetaIntegrator{
		cfg:    cfg,
		Client: client,
	}
}

func (s *MetaIntegrator) GetAdAccountReachImpressions(accountID string, filters *domain.InsigthFilters) (*domain.ReachImpressionsResponse, error) {
	params := &url.Values{}
	params.Add("fields", "account_id,account_name, impressions, reach, frequency")

	resp, err := s.Client.GetAdAccountInsightsByID(accountID, filters, params)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"account_id": accountID,
			"error":      err.Error(),
		}).Error("insights: failed to get ad account insights from API")
		return nil, err
	}

	adAccountMetrics := FactoryAdAccountMetrics(resp)
	if adAccountMetrics == nil {
		logrus.WithField("account_id", accountID).Error("insights: failed to convert ad account metrics")
		return nil, fmt.Errorf("Error factory ad account metrics")
	}

	logrus.WithFields(logrus.Fields{
		"account_id":   accountID,
		"account_name": adAccountMetrics.Name,
	}).Debug("insights: successfully retrieved ad account metrics")

	return &domain.ReachImpressionsResponse{
		AccountID:   accountID,
		AccountName: adAccountMetrics.Name,
		Reach:       adAccountMetrics.Reach,
		Impressions: adAccountMetrics.Impressions,
		StartDate:   filters.StartDate.Format(time.DateOnly),
		EndDate:     filters.EndDate.Format(time.DateOnly),
	}, nil
}

func (s *MetaIntegrator) GetAdAccountsInsights(accountID string, filters *domain.InsigthFilters) (*domain.AdAccountMetrics, error) {
	params := &url.Values{}
	params.Add("fields", "account_id,account_name,spend,actions,cost_per_action_type, objective, impressions, reach, frequency")

	resp, err := s.Client.GetAdAccountInsightsByID(accountID, filters, params)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"account_id": accountID,
			"error":      err.Error(),
		}).Error("insights: failed to get ad account insights from API")
		return nil, err
	}

	adAccountMetrics := FactoryAdAccountMetrics(resp)
	if adAccountMetrics == nil {
		logrus.WithField("account_id", accountID).Error("insights: failed to convert ad account metrics")
		return nil, fmt.Errorf("Error factory ad account metrics")
	}

	logrus.WithFields(logrus.Fields{
		"account_id":   accountID,
		"account_name": adAccountMetrics.Name,
	}).Debug("insights: successfully retrieved ad account metrics")

	campaigns, err := s.Client.GetAdCampaignByAccountID(accountID)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"account_id": accountID,
			"error":      err.Error(),
		}).Error("insights: failed to get campaigns for ad account")
	}

	campaignsInsights := make([]*domain.CampaignInsight, 0)
	AccountResult := 0
	AccountSpend := 0.0
	for _, campaign := range campaigns {
		campaignInsight, err := s.Client.GetAdCampaignInsightsByID(campaign.ID, filters)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"campaign_id": campaign.ID,
				"account_id":  accountID,
				"error":       err.Error(),
			}).Error("insights: failed to get campaign insights")
			continue
		}

		result := campaignInsight.GetResult()
		costPerResult := campaignInsight.GetCostPerResult()

		spend, err := strconv.ParseFloat(campaignInsight.Spend, 64)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"campaign_id": campaign.ID,
				"spend_value": campaignInsight.Spend,
				"error":       err.Error(),
			}).Warn("insights: error converting spend to float")
		}

		if result > 0 && spend > 0 {
			AccountResult += result
			AccountSpend += spend
		}

		cp := &domain.CampaignInsight{
			CampaignID:    campaignInsight.CampaignID,
			CampaignName:  campaignInsight.CampaignName,
			Clicks:        campaignInsight.Clicks,
			Frequency:     campaignInsight.Frequency,
			Impressions:   campaignInsight.Impressions,
			Objective:     campaignInsight.Objective,
			Reach:         campaignInsight.Reach,
			Spend:         spend,
			Result:        result,
			CostPerResult: costPerResult,
		}

		campaignsInsights = append(campaignsInsights, cp)
	}

	var costPerResult float64
	if AccountResult > 0 {
		costPerResult = AccountSpend / float64(AccountResult)
	}

	return &domain.AdAccountMetrics{
		AdAccountInsight: domain.AdAccountInsight{
			AccountID:     adAccountMetrics.AccountID,
			Name:          adAccountMetrics.Name,
			Spend:         adAccountMetrics.Spend,
			Objective:     adAccountMetrics.Objective,
			Reach:         adAccountMetrics.Reach,
			Impressions:   adAccountMetrics.Impressions,
			Frequency:     adAccountMetrics.Frequency,
			Campaigns:     campaignsInsights,
			Result:        AccountResult,
			CostPerResult: utils.RoundWithTwoDecimalPlace(costPerResult),
		},
	}, nil
}

func (s *MetaIntegrator) GetAdAccounts() ([]*domain.AdAccount, error) {
	bms, err := s.getBusinessManagers()
	if err != nil {
		logrus.WithError(err).Error("insights: failed to get business managers")
		return nil, err
	}

	allAdAccounts := make([]*domain.AdAccount, 0)
	for _, b := range bms {
		logrus.WithFields(logrus.Fields{
			"business_id":   b.ID,
			"business_name": b.Name,
		}).Debug("insights: fetching ad accounts for business")

		adAccounts, err := s.Client.GetAdAccountsByBusinessID(b.ID)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"business_id": b.ID,
				"error":       err.Error(),
			}).Error("insights: failed to get ad accounts for business")
			continue
		}

		for _, adAccount := range adAccounts {
			allAdAccounts = append(allAdAccounts, &domain.AdAccount{
				ExternalID:          adAccount.ID,
				Name:                adAccount.Name,
				Nickname:            &adAccount.Name,
				Origin:              "meta",
				BusinessManagerID:   b.ID,
				BusinessManagerName: b.Name,
			})
		}
	}

	logrus.WithField("total_accounts", len(allAdAccounts)).Info("insights: successfully retrieved all ad accounts")

	return allAdAccounts, nil
}

func (s *MetaIntegrator) getBusinessManagers() ([]metadomain.BusinessManager, error) {
	if err := s.Client.EnsureValidToken(); err != nil {
		return nil, fmt.Errorf("erro ao verificar validade do token: %w", err)
	}

	url := fmt.Sprintf("%s/me/businesses?limit=100&access_token=%s", s.cfg.Meta.URL, s.cfg.Meta.AccessToken)

	data, err := utils.MakeRequest(url)
	if err != nil {
		if strings.Contains(err.Error(), "Error on Request") {
			if refreshErr := s.Client.RefreshToken(); refreshErr != nil {
				return nil, fmt.Errorf("erro ao renovar token: %w", refreshErr)
			}

			url = fmt.Sprintf("%s/me/businesses?limit=100&access_token=%s", s.cfg.Meta.URL, s.cfg.Meta.AccessToken)

			data, err = utils.MakeRequest(url)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	var response struct {
		Data []metadomain.BusinessManager `json:"data"`
	}
	err = json.Unmarshal(data, &response)
	if err != nil {
		return nil, err
	}

	return response.Data, nil
}

func FactoryAdAccountMetrics(adAccountInsight *metadomain.AdAccountInsight) *metadomain.AdAccountMetrics {
	actions := make(map[string]float64)

	for i := range len(adAccountInsight.Actions) {
		action := adAccountInsight.Actions[i]

		actionValue, err := strconv.ParseFloat(action.Value, 64)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"action_type":  action.ActionType,
				"action_value": action.Value,
				"error":        err.Error(),
			}).Warn("insights: error converting action value to float")
			continue
		}

		actions[action.ActionType] = actionValue
	}

	spend, err := strconv.ParseFloat(adAccountInsight.Spend, 64)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"spend_value": adAccountInsight.Spend,
			"error":       err.Error(),
		}).Warn("insights: error converting spend to float")
	}

	frequency, err := strconv.ParseFloat(adAccountInsight.Frequency, 64)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"frequency_value": adAccountInsight.Frequency,
			"error":           err.Error(),
		}).Warn("insights: error converting frequency to float")
	}

	reach, err := strconv.Atoi(adAccountInsight.Reach)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"reach_value": adAccountInsight.Reach,
			"error":       err.Error(),
		}).Warn("insights: error converting reach to integer")
	}

	impressions, err := strconv.Atoi(adAccountInsight.Impressions)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"impressions_value": adAccountInsight.Impressions,
			"error":             err.Error(),
		}).Warn("insights: error converting impressions to integer")
	}

	return &metadomain.AdAccountMetrics{
		AccountID:   adAccountInsight.AccountID,
		Name:        adAccountInsight.Name,
		Spend:       spend,
		Actions:     actions,
		Objective:   adAccountInsight.Objective,
		Reach:       reach,
		Impressions: impressions,
		Frequency:   frequency,
	}
}
