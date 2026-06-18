package dto

type UserResultDTO struct {
	Code    int     `json:"code" example:"0"`
	Message string  `json:"message" example:"success"`
	Data    UserDTO `json:"data"`
	Error   string  `json:"error,omitempty" example:"error details"`
}

type ResetAccessResultDTO struct {
	Code    int                    `json:"code" example:"0"`
	Message string                 `json:"message" example:"success"`
	Data    ResetAccessResponseDTO `json:"data"`
	Error   string                 `json:"error,omitempty" example:"error details"`
}

type MFAStatusResultDTO struct {
	Code    int          `json:"code" example:"0"`
	Message string       `json:"message" example:"success"`
	Data    MFAStatusDTO `json:"data"`
	Error   string       `json:"error,omitempty" example:"error details"`
}

type PasskeyListResultDTO struct {
	Code    int                    `json:"code" example:"0"`
	Message string                 `json:"message" example:"success"`
	Data    PasskeyListResponseDTO `json:"data"`
	Error   string                 `json:"error,omitempty" example:"error details"`
}

type PasskeyOptionsResultDTO struct {
	Code    int                       `json:"code" example:"0"`
	Message string                    `json:"message" example:"success"`
	Data    PasskeyOptionsResponseDTO `json:"data"`
	Error   string                    `json:"error,omitempty" example:"error details"`
}

type PasskeyCredentialResultDTO struct {
	Code    int                         `json:"code" example:"0"`
	Message string                      `json:"message" example:"success"`
	Data    PasskeyCredentialSummaryDTO `json:"data"`
	Error   string                      `json:"error,omitempty" example:"error details"`
}
