package authenticating

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
	"github.com/vfg2006/traffic-manager-api/infrastructure/repository"
	errorcodes "github.com/vfg2006/traffic-manager-api/internal/api/errors"
	"github.com/vfg2006/traffic-manager-api/internal/config"
	"github.com/vfg2006/traffic-manager-api/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

var secretKey = "seu_segredo_super_secreto"

type Authenticator interface {
	CreateUser(user *domain.User) (*domain.User, error)
	UpdateUser(user *domain.UpdateUserRequest) error
	ListUser() ([]*domain.User, error)
	LoginUser(email, password string) (string, error)
	GetUserProfile(userID int) (*domain.User, error)
	ValidateToken(tokenString string) (*domain.Claims, error)
	GenerateStrongPassword(requestUserID, targetUserID int) (string, error)
	ChangePassword(userID int, currentPassword, newPassword string) error
	ValidatePasswordStrength(password string) error
	GetUserLinkedAccounts(userID int) ([]*domain.AdAccountResponse, error)
	LinkUserAccount(userID int, accountID string) error
	UnlinkUserAccount(userID int, accountID string) error
	ManageUserAccounts(userID int, accountIDs []string) error
}

type Service struct {
	userRepo    repository.UserRepository
	accountRepo repository.AccountRepository
	cfg         *config.Config
}

func NewService(userRepo repository.UserRepository, accountRepo repository.AccountRepository, cfg *config.Config) Authenticator {
	return &Service{
		userRepo:    userRepo,
		accountRepo: accountRepo,
		cfg:         cfg,
	}
}

func (s *Service) UpdateUser(user *domain.UpdateUserRequest) error {
	if user.ID == 0 {
		return errors.New("ID is required")
	}

	userDatabase, err := s.userRepo.GetUserByID(user.ID)
	if userDatabase == nil || err != nil {
		if err == nil {
			return errors.New(fmt.Sprintf("user not found to ID: %d", user.ID))
		}
		return err
	}

	if user.Name != nil {
		userDatabase.Name = *user.Name
	}

	if user.Lastname != nil {
		userDatabase.Lastname = *user.Lastname
	}

	if user.Email != nil {
		userDatabase.Email = *user.Email
	}

	if user.Active != nil {
		userDatabase.Active = *user.Active
	}

	if user.RoleID != nil {
		userDatabase.RoleID = *user.RoleID
	}

	if user.AvatarURL != nil {
		userDatabase.AvatarURL = user.AvatarURL
	}

	if user.Deleted != nil {
		now := time.Now()
		userDatabase.Deleted = *user.Deleted
		userDatabase.DeletedAt = &now
	}

	err = s.userRepo.UpdateUser(userDatabase)
	if err != nil {
		return err
	}

	return nil
}

func (s *Service) CreateUser(user *domain.User) (*domain.User, error) {
	if user.Email == "" || user.Name == "" || user.Lastname == "" || user.PasswordHash == "" {
		return nil, NewAuthError(ErrMissingRequiredData, errorcodes.ErrMissingRequiredData, "Email, nome, sobrenome e senha são obrigatórios")
	}

	user.Email = handleEmail(user.Email)

	userDatabase, err := s.userRepo.GetUserByEmail(user.Email)
	if userDatabase != nil {
		return nil, NewAuthError(ErrUserAlreadyExists, errorcodes.ErrUserAlreadyExists, "Email já cadastrado")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.PasswordHash), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	if user.RoleID == 0 {
		user.RoleID = 3
	}

	user.PasswordHash = string(hashedPassword)
	user.Active = false

	user, err = s.userRepo.CreateUser(user)
	if err != nil {
		return nil, NewAuthError(err, errorcodes.ErrDatabaseOperation, "Erro ao criar usuário")
	}

	return user, nil
}

func handleEmail(s string) string {
	email := strings.ToLower(s)
	email = strings.TrimSpace(email)
	email = strings.ReplaceAll(email, " ", "")
	return email
}

func (s *Service) ListUser() ([]*domain.User, error) {
	users, err := s.userRepo.ListUser()
	if err != nil {
		return nil, err
	}

	return users, nil
}

func (s *Service) LoginUser(email, password string) (string, error) {
	// Validação de entrada
	if email == "" || password == "" {
		return "", NewAuthError(ErrMissingRequiredData, errorcodes.ErrUserDisabled, "Email e senha são obrigatórios")
	}

	email = handleEmail(email)

	user, err := s.userRepo.GetUserByEmail(email)
	if err != nil {
		return "", NewAuthError(err, errorcodes.ErrDatabaseOperation, "Erro ao consultar usuário no banco de dados")
	}

	// Verificar se o usuário existe
	if user == nil {
		return "", NewAuthError(ErrUserNotFound, errorcodes.ErrUserNotFound, "Usuário não encontrado")
	}

	// Verificar se o usuário está ativo
	if !user.Active {
		return "", NewUserAuthError(ErrUserDisabled, errorcodes.ErrUserDisabled, user.ID, "Conta desativada")
	}

	// Verificar senha
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", NewUserAuthError(ErrInvalidCredentials, errorcodes.ErrInvalidCredentials, user.ID, "Senha incorreta")
	}

	// Gerar token JWT
	token, err := generateJWT(user, s.cfg.SecretKey)
	if err != nil {
		return "", NewAuthError(err, errorcodes.ErrInternalServer, "Erro ao gerar token de autenticação")
	}

	return token, nil
}

func (s *Service) GetUserProfile(userID int) (*domain.User, error) {
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		logrus.Error(err)
		return nil, err
	}

	user.PasswordHash = ""
	return user, nil
}

func generateJWT(user *domain.User, secretKey string) (string, error) {
	claims := domain.Claims{
		UserID:        user.ID,
		UserName:      user.Name,
		UserLastname:  user.Lastname,
		UserEmail:     user.Email,
		UserActive:    user.Active,
		UserRoleID:    user.RoleID,
		UserAvatarURL: user.AvatarURL,
		UserAccounts:  user.LinkedAccounts,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

func (s *Service) ValidateToken(tokenString string) (*domain.Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &domain.Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.cfg.SecretKey), nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*domain.Claims); ok && token.Valid {
		return claims, nil
	} else {
		return nil, errors.New("invalid token")
	}
}

// GenerateStrongPassword gera uma senha forte para o usuário alvo.
// Verifica se o usuário solicitante tem perfil de administrador (role_id = 1) antes de prosseguir.
func (s *Service) GenerateStrongPassword(requestUserID, targetUserID int) (string, error) {
	// Verificar se o usuário solicitante é um administrador
	requestUser, err := s.userRepo.GetUserByID(requestUserID)
	if err != nil {
		return "", err
	}
	if requestUser == nil {
		return "", errors.New("usuário solicitante não encontrado")
	}
	if requestUser.RoleID != 1 {
		return "", errors.New("apenas administradores podem gerar novas senhas")
	}

	// Verificar se o usuário alvo existe
	targetUser, err := s.userRepo.GetUserByID(targetUserID)
	if err != nil {
		return "", err
	}
	if targetUser == nil {
		return "", errors.New("usuário alvo não encontrado")
	}

	// Gerar senha forte
	newPassword, err := generateStrongPassword(12)
	if err != nil {
		return "", err
	}

	// Hash da nova senha
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	// Atualizar senha do usuário alvo
	targetUser.PasswordHash = string(hashedPassword)
	err = s.userRepo.UpdateUser(targetUser)
	if err != nil {
		return "", err
	}

	return newPassword, nil
}

// generateStrongPassword gera uma senha forte com o comprimento especificado
// incluindo letras maiúsculas, minúsculas, números e caracteres especiais
func generateStrongPassword(length int) (string, error) {
	if length < 8 {
		length = 8 // Comprimento mínimo para senhas fortes
	}

	const (
		lowerChars   = "abcdefghijklmnopqrstuvwxyz"
		upperChars   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		numberChars  = "0123456789"
		specialChars = "!@#$%^&*()-_=+[]{}|;:,.<>?"
		allChars     = lowerChars + upperChars + numberChars + specialChars
	)

	// Garantir que a senha tenha pelo menos um caractere de cada tipo
	password := make([]byte, length)

	// Adicionar um caractere minúsculo
	randomChar, err := getRandomChar(lowerChars)
	if err != nil {
		return "", err
	}
	password[0] = randomChar

	// Adicionar um caractere maiúsculo
	randomChar, err = getRandomChar(upperChars)
	if err != nil {
		return "", err
	}
	password[1] = randomChar

	// Adicionar um número
	randomChar, err = getRandomChar(numberChars)
	if err != nil {
		return "", err
	}
	password[2] = randomChar

	// Adicionar um caractere especial
	randomChar, err = getRandomChar(specialChars)
	if err != nil {
		return "", err
	}
	password[3] = randomChar

	// Preencher o resto com caracteres aleatórios
	for i := 4; i < length; i++ {
		randomChar, err = getRandomChar(allChars)
		if err != nil {
			return "", err
		}
		password[i] = randomChar
	}

	// Embaralhar a senha para que os caracteres não fiquem em ordem previsível
	for i := range password {
		j, err := randomInt(int64(len(password)))
		if err != nil {
			return "", err
		}
		password[i], password[j] = password[j], password[i]
	}

	return string(password), nil
}

// getRandomChar retorna um caractere aleatório do conjunto fornecido
func getRandomChar(charset string) (byte, error) {
	n, err := randomInt(int64(len(charset)))
	if err != nil {
		return 0, err
	}
	return charset[n], nil
}

// randomInt gera um número aleatório seguro entre 0 e max-1
func randomInt(max int64) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

// ValidatePasswordStrength verifica se a senha atende aos requisitos de segurança
// Senha deve conter pelo menos 8 caracteres, incluindo maiúsculas, minúsculas, números e caracteres especiais
func (s *Service) ValidatePasswordStrength(password string) error {
	if len(password) < 8 {
		return errors.New("a senha deve conter pelo menos 8 caracteres")
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	const (
		lowerChars   = "abcdefghijklmnopqrstuvwxyz"
		upperChars   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		numberChars  = "0123456789"
		specialChars = "!@#$%^&*()-_=+[]{}|;:,.<>?"
	)

	for _, char := range password {
		switch {
		case strings.ContainsRune(lowerChars, char):
			hasLower = true
		case strings.ContainsRune(upperChars, char):
			hasUpper = true
		case strings.ContainsRune(numberChars, char):
			hasNumber = true
		case strings.ContainsRune(specialChars, char):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return errors.New("a senha deve conter pelo menos uma letra maiúscula")
	}
	if !hasLower {
		return errors.New("a senha deve conter pelo menos uma letra minúscula")
	}
	if !hasNumber {
		return errors.New("a senha deve conter pelo menos um número")
	}
	if !hasSpecial {
		return errors.New("a senha deve conter pelo menos um caractere especial")
	}

	return nil
}

// ChangePassword permite que um usuário altere sua própria senha
// Verifica se a senha atual está correta e se a nova senha atende aos requisitos de segurança
func (s *Service) ChangePassword(userID int, currentPassword, newPassword string) error {
	// Obter o usuário pelo ID
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return err
	}

	if user == nil {
		return errors.New("usuário não encontrado")
	}

	// Verificar se a senha atual está correta
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return errors.New("senha atual incorreta")
	}

	// Validar se a nova senha atende aos requisitos de segurança
	if err := s.ValidatePasswordStrength(newPassword); err != nil {
		return err
	}

	// Gerar hash da nova senha
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Atualizar a senha do usuário
	user.PasswordHash = string(hashedPassword)
	err = s.userRepo.UpdateUser(user)
	if err != nil {
		return err
	}

	return nil
}

// GetUserLinkedAccounts retorna as contas vinculadas a um usuário
func (s *Service) GetUserLinkedAccounts(userID int) ([]*domain.AdAccountResponse, error) {
	accountIDs, err := s.userRepo.GetUserLinkedAccounts(userID)
	if err != nil {
		return nil, err
	}

	accounts := make([]*domain.AdAccountResponse, 0)
	for _, id := range accountIDs {
		account, err := s.accountRepo.GetAccountByID(id)
		if err != nil {
			return nil, err
		}

		if account == nil || account.Status != domain.AdAccountStatusActive {
			continue
		}

		accounts = append(accounts, &domain.AdAccountResponse{
			ID:         account.ID,
			Name:       account.Name,
			ExternalID: account.ExternalID,
			HasToken:   account.SecretName != nil,
			Status:     account.Status,
			CNPJ:       account.CNPJ,
			Nickname:   account.Nickname,
		})
	}

	return accounts, nil
}

// LinkUserAccount adiciona um vínculo entre usuário e conta
func (s *Service) LinkUserAccount(userID int, accountID string) error {
	// Verificar se o usuário existe
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("usuário não encontrado")
	}

	// Verificar se a conta existe
	// Aqui precisaria de acesso ao repositório de contas
	// Por simplicidade, apenas adicionamos o vínculo

	return s.userRepo.LinkUserAccount(userID, accountID)
}

// UnlinkUserAccount remove o vínculo entre usuário e conta
func (s *Service) UnlinkUserAccount(userID int, accountID string) error {
	// Verificar se o usuário existe
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("usuário não encontrado")
	}

	return s.userRepo.UnlinkUserAccount(userID, accountID)
}

// ManageUserAccounts atualiza todas as contas vinculadas a um usuário
// Isso remove todas as existentes e adiciona as novas
func (s *Service) ManageUserAccounts(userID int, accountIDs []string) error {
	// Verificar se o usuário existe
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("usuário não encontrado")
	}

	// Obter contas atuais
	currentAccounts, err := s.userRepo.GetUserLinkedAccounts(userID)
	if err != nil {
		return err
	}

	// Remover contas que não estão na nova lista
	for _, current := range currentAccounts {
		found := false
		for _, new := range accountIDs {
			if current == new {
				found = true
				break
			}
		}

		if !found {
			err := s.userRepo.UnlinkUserAccount(userID, current)
			if err != nil {
				logrus.Warnf("Erro ao desvincular conta %s do usuário %d: %v", current, userID, err)
				// Continuar mesmo com erro
			}
		}
	}

	// Adicionar novas contas
	for _, new := range accountIDs {
		found := false
		for _, current := range currentAccounts {
			if current == new {
				found = true
				break
			}
		}

		if !found {
			err := s.userRepo.LinkUserAccount(userID, new)
			if err != nil {
				logrus.Warnf("Erro ao vincular conta %s ao usuário %d: %v", new, userID, err)
				// Continuar mesmo com erro
			}
		}
	}

	return nil
}
