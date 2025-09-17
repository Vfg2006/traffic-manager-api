package metadomain

// ErrorResponse representa a estrutura de erro da API do Meta
type ErrorResponse struct {
	Error ErrorDetails `json:"error"`
}

// ErrorDetails contém os detalhes de erro da API do Meta
type ErrorDetails struct {
	Message      string      `json:"message"`
	Type         string      `json:"type"`
	Code         int         `json:"code"`
	ErrorSubcode int         `json:"error_subcode,omitempty"`
	FBTraceID    string      `json:"fbtrace_id"`
	ErrorData    interface{} `json:"error_data,omitempty"`
}

// IsTokenExpired verifica se o erro é de token expirado
func (e *ErrorResponse) IsTokenExpired() bool {
	// O código 190 representa "token expirado" nas respostas da API do Meta
	// Possíveis subcódigos relacionados a problemas de token: 460, 463, 467
	return e.Error.Code == 190 ||
		(e.Error.Type == "OAuthException" && (e.Error.ErrorSubcode == 460 || e.Error.ErrorSubcode == 463 || e.Error.ErrorSubcode == 467))
}
