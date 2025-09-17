package metadomain

import (
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/vfg2006/traffic-manager-api/pkg/utils"
)

type Campaign struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type Cursors struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

type Paging struct {
	Cursors Cursors `json:"cursors"`
}

type CampaignInsight struct {
	AccountID      string   `json:"account_id"`
	AccountName    string   `json:"account_name"`
	Actions        []Action `json:"actions"`
	CampaignID     string   `json:"campaign_id"`
	CampaignName   string   `json:"campaign_name"`
	Clicks         string   `json:"clicks"`
	CostPerActions []Action `json:"cost_per_action_type"`
	DateStart      string   `json:"date_start"`
	DateStop       string   `json:"date_stop"`
	Frequency      string   `json:"frequency"`
	Impressions    string   `json:"impressions"`
	Objective      string   `json:"objective"`
	Reach          string   `json:"reach"`
	Spend          string   `json:"spend"`
}

func (c *CampaignInsight) GetResult() int {
	for i := range len(c.Actions) {
		action := c.Actions[i]

		if _, ok := MetaObjectiveToActionType[c.Objective]; !ok {
			logrus.Info("Objective not mapped: ", c.Objective)
		}

		if action.ActionType == MetaObjectiveToActionType[c.Objective] {
			actionValue, err := strconv.Atoi(action.Value)
			if err != nil {
				logrus.WithError(err).Error("Erro ao converter valor da ação")
			}

			return actionValue
		}
	}

	logrus.WithField("objective", c.Objective).Warn("Ação não encontrada")
	logrus.WithField("actions", c.Actions).Debug("Ações disponíveis")

	return 0
}

func (c *CampaignInsight) GetCostPerResult() float64 {
	for i := range len(c.CostPerActions) {
		action := c.CostPerActions[i]

		if action.ActionType == MetaObjectiveToActionType[c.Objective] {
			actionValue, err := strconv.ParseFloat(action.Value, 64)
			if err != nil {
				logrus.WithError(err).Error("Erro ao converter valor do custo por ação")
			}

			return utils.RoundWithTwoDecimalPlace(actionValue)
		}
	}

	logrus.WithField("objective", c.Objective).Warn("Custo por resultado não encontrado")
	logrus.WithField("cost_per_actions", c.CostPerActions).Debug("Custos por ação disponíveis")

	return 0
}
