package ssoticadomain

type OrderItem struct {
	ID             int          `json:"id,omitempty"`
	Product        Product      `json:"produto,omitempty"`
	Quantity       float64      `json:"quantidade,omitempty"`
	Cost           float64      `json:"custo,omitempty"`
	GrossUnitPrice float64      `json:"valor_unitario_bruto,omitempty"`
	Discount       float64      `json:"desconto,omitempty"`
	Increase       float64      `json:"acrescimo,omitempty"`
	NetUnitPrice   float64      `json:"valor_unitario_liquido,omitempty"`
	NetTotalPrice  float64      `json:"valor_total_liquido,omitempty"`
	ServiceOrder   OrderService `json:"ordem_servico,omitempty"`
}

type Product struct {
	ID          int         `json:"id,omitempty"`
	Reference   string      `json:"referencia,omitempty"`
	Description string      `json:"descricao,omitempty"`
	Group       string      `json:"grupo,omitempty"`
	GroupID     int         `json:"grupo_id,omitempty"`
	Brand       string      `json:"grife,omitempty"`
	BrandID     int         `json:"grife_id,omitempty"`
	Unit        string      `json:"unidade,omitempty"`
	GtinCode    interface{} `json:"código_gtin,omitempty"`
	Ncm         string      `json:"ncm,omitempty"`
}

type OrderService struct {
	ID                int    `json:"id,omitempty"`
	Type              string `json:"tipo,omitempty"`
	Number            int    `json:"numero,omitempty"`
	Status            string `json:"status,omitempty"`
	DetailedStatus    string `json:"status_detalhado,omitempty"`
	OpeningDate       string `json:"abertura,omitempty"`
	EstimatedDelivery string `json:"previsao_entrega,omitempty"`
}
