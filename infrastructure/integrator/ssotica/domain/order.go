package ssoticadomain

import (
	"slices"
	"time"
)

type Origin string

const (
	SocialNetworkOrigin Origin = "Redes Sociais"
	OthersOrigin        Origin = "others"
)

var SocialNetworkOrigins = []Origin{
	SocialNetworkOrigin,
	"Tráfego Pago",
	"Rede Social",
	"Redes Sociais / Trafego Pago",
	"Redes Sociais / Trafego Pago",
	"Trafego Pago",
	"Redes Sociais / Trafego",
	"Redes Socias",
	"Rede Social - Eclel",
	"Rede Social - Bruna",
}

type Order struct {
	ID             int             `json:"id,omitempty"`
	Date           string          `json:"data,omitempty"`
	Time           string          `json:"hora,omitempty"`
	Status         string          `json:"status,omitempty"`
	Number         int             `json:"numero,omitempty"`
	GrossAmount    float64         `json:"valor_bruto,omitempty"`
	Increase       float64         `json:"acrescimo,omitempty"`
	Discount       float64         `json:"desconto,omitempty"`
	ExchangeCredit float64         `json:"credito_troca,omitempty"`
	NetAmount      float64         `json:"valor_liquido,omitempty"`
	Items          []OrderItem     `json:"itens,omitempty"`
	PaymentMethods []PaymentMethod `json:"formas_pagamento,omitempty"`
	// Customer        Customer        `json:"cliente,omitempty"`
	Employee        Employee `json:"funcionario,omitempty"`
	CustomerOrigins []Origin `json:"origensCliente,omitempty"`
}

type PaymentMethod struct {
	ID                int         `json:"id,omitempty"`
	Date              string      `json:"data,omitempty"`
	Value             string      `json:"valor,omitempty"`
	InstallmentsCount int         `json:"qtd_parcelas,omitempty"`
	PaymentMethod     string      `json:"forma_pagamento,omitempty"`
	AuthorizationCode interface{} `json:"codigo_autorizacao,omitempty"`
}

type Customer struct {
	ID                int      `json:"id,omitempty"`
	Name              string   `json:"nome,omitempty"`
	Nickname          *string  `json:"apelido,omitempty"`
	BirthDate         string   `json:"nascimento,omitempty"`
	CpfCnpj           string   `json:"cpf_cnpj,omitempty"`
	RegisterNumber    *string  `json:"rg_ie,omitempty"`
	FatherName        *string  `json:"nome_pai,omitempty"`
	MotherName        *string  `json:"nome_mae,omitempty"`
	Profession        *string  `json:"profissao,omitempty"`
	Observation       *string  `json:"observacao,omitempty"`
	IsActive          bool     `json:"ativo,omitempty"`
	IsICMSContributor bool     `json:"contribuinte_icms,omitempty"`
	Suframa           *string  `json:"suframa,omitempty"`
	Ie                *string  `json:"ie,omitempty"`
	Im                *string  `json:"im,omitempty"`
	Type              string   `json:"tipo,omitempty"`
	RegisteredAt      string   `json:"cadastrado_em,omitempty"`
	Gender            *string  `json:"sexo,omitempty"`
	FamilyIncome      *string  `json:"renda_familiar,omitempty"`
	Code              *string  `json:"codigo,omitempty"`
	Agreement         string   `json:"convenio,omitempty"`
	Education         *string  `json:"escolaridade,omitempty"`
	Reference         *string  `json:"referencia,omitempty"`
	Address           Address  `json:"endereco,omitempty"`
	Phones            []Phone  `json:"telefones,omitempty"`
	Emails            []string `json:"emails,omitempty"`
	Origin            string   `json:"origem,omitempty"`
}

type Address struct {
	Street       string `json:"logradouro,omitempty"`
	Number       string `json:"numero,omitempty"`
	Complement   string `json:"complemento,omitempty"`
	Neighborhood string `json:"bairro,omitempty"`
	PostalCode   string `json:"cep,omitempty"`
	City         string `json:"cidade,omitempty"`
	UF           string `json:"uf,omitempty"`
	Country      string `json:"pais,omitempty"`
}

type Phone struct {
	Number         string      `json:"numero,omitempty"`
	Identification interface{} `json:"identificacao,omitempty"`
}

type Employee struct {
	ID             int     `json:"id,omitempty"`
	Name           string  `json:"nome,omitempty"`
	CPF            string  `json:"cpf,omitempty"`
	CNPJ           string  `json:"cnpj,omitempty"`
	RegisterNumber string  `json:"rg,omitempty"`
	HomePhone      string  `json:"telefone_fixo,omitempty"`
	MobilePhone    string  `json:"telefone_movel,omitempty"`
	Address        Address `json:"endereco,omitempty"`
	Role           string  `json:"funcao,omitempty"`
	Observation    string  `json:"observacao,omitempty"`
}

type GetSalesParams struct {
	CNPJ       string
	SecretName string
}

type CheckConnectionParams struct {
	CNPJ      string
	Token     string
	StartDate time.Time
	EndDate   time.Time
}

func GetSumNetAmountSocialNetwork(s []Order) float64 {
	var totalNetAmount float64

	for _, sale := range s {
		if len(sale.CustomerOrigins) > 0 && slices.Contains(SocialNetworkOrigins, sale.CustomerOrigins[0]) {
			totalNetAmount += sale.NetAmount
		}
	}

	return totalNetAmount
}
