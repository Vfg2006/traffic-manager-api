package domain

// AvailablePeriods representa os períodos mensais disponíveis nas tabelas de insights
type AvailablePeriods struct {
	Periods []string `json:"periods"` // Lista de períodos no formato mm-yyyy
	Years   []string `json:"years"`   // Lista de anos únicos disponíveis
	Months  []string `json:"months"`  // Lista de meses únicos disponíveis
}
