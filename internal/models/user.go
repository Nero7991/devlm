package models

import (
	"errors"
	"regexp"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	ID              uuid.UUID `gorm:"type:uuid;primary_key"`
	Username        string    `gorm:"uniqueIndex;not null"`
	Email           string    `gorm:"uniqueIndex;not null"`
	Password        string    `gorm:"not null"`
	FirstName       string
	LastName        string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	LastLoginAt     *time.Time
	PasswordHistory []string `gorm:"type:text[]"`
	Role            string   `gorm:"default:'user'"`
}

func NewUser(username, email, password, firstName, lastName string) (*User, error) {
	if err := validateInput(username, email, password); err != nil {
		return nil, err
	}

	hashedPassword, err := hashPassword(password)
	if err != nil {
		return nil, err
	}

	return &User{
		ID:              uuid.New(),
		Username:        username,
		Email:           email,
		Password:        hashedPassword,
		FirstName:       firstName,
		LastName:        lastName,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		PasswordHistory: []string{hashedPassword},
		Role:            "user",
	}, nil
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	u.CreatedAt = time.Now()
	u.UpdatedAt = time.Now()
	return nil
}

func (u *User) BeforeUpdate(tx *gorm.DB) error {
	u.UpdatedAt = time.Now()
	return nil
}

func (u *User) FullName() string {
	if u.FirstName == "" && u.LastName == "" {
		return u.Username
	}
	return u.FirstName + " " + u.LastName
}

func (u *User) VerifyPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

func (u *User) UpdateProfile(firstName, lastName string) error {
	if len(firstName) > 50 || len(lastName) > 50 {
		return errors.New("first name and last name must not exceed 50 characters")
	}
	u.FirstName = firstName
	u.LastName = lastName
	return nil
}

func (u *User) ChangePassword(currentPassword, newPassword string) error {
	if !u.VerifyPassword(currentPassword) {
		return errors.New("current password is incorrect")
	}

	if err := validatePassword(newPassword); err != nil {
		return err
	}

	if u.isPasswordInHistory(newPassword) {
		return errors.New("new password must not be one of the last 5 passwords used")
	}

	hashedPassword, err := hashPassword(newPassword)
	if err != nil {
		return err
	}

	u.Password = hashedPassword
	u.addPasswordToHistory(hashedPassword)
	return nil
}

func (u *User) isPasswordInHistory(password string) bool {
	for _, historicalPassword := range u.PasswordHistory {
		if bcrypt.CompareHashAndPassword([]byte(historicalPassword), []byte(password)) == nil {
			return true
		}
	}
	return false
}

func (u *User) addPasswordToHistory(hashedPassword string) {
	u.PasswordHistory = append(u.PasswordHistory, hashedPassword)
	if len(u.PasswordHistory) > 5 {
		u.PasswordHistory = u.PasswordHistory[len(u.PasswordHistory)-5:]
	}
}

func (u *User) UpdateLastLogin() {
	now := time.Now()
	u.LastLoginAt = &now
}

func validateInput(username, email, password string) error {
	if len(username) < 3 || len(username) > 30 {
		return errors.New("username must be between 3 and 30 characters long")
	}

	usernameRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !usernameRegex.MatchString(username) {
		return errors.New("username can only contain letters, numbers, underscores, and hyphens")
	}

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return errors.New("invalid email format")
	}

	return validatePassword(password)
}

func validatePassword(password string) error {
	if len(password) < 12 {
		return errors.New("password must be at least 12 characters long")
	}

	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasNumber := regexp.MustCompile(`[0-9]`).MatchString(password)
	hasSpecial := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};:'",.<>?~]`).MatchString(password)

	if !hasUpper || !hasLower || !hasNumber || !hasSpecial {
		return errors.New("password must contain at least one uppercase letter, one lowercase letter, one number, and one special character")
	}

	return nil
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func GetUserByID(db *gorm.DB, id uuid.UUID) (*User, error) {
	var user User
	result := db.First(&user, "id = ?", id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

func GetUserByUsername(db *gorm.DB, username string) (*User, error) {
	var user User
	result := db.First(&user, "username = ?", username)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

func GetUserByEmail(db *gorm.DB, email string) (*User, error) {
	var user User
	result := db.First(&user, "email = ?", email)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

func DeleteUser(db *gorm.DB, id uuid.UUID) error {
	result := db.Delete(&User{}, "id = ?", id)
	return result.Error
}

func GetAllUsers(db *gorm.DB, limit, offset int) ([]User, error) {
	var users []User
	result := db.Limit(limit).Offset(offset).Find(&users)
	if result.Error != nil {
		return nil, result.Error
	}
	return users, nil
}

func (u *User) Save(db *gorm.DB) error {
	return db.Save(u).Error
}

func (u *User) SetRole(role string) error {
	validRoles := map[string]bool{"user": true, "admin": true, "moderator": true}
	if !validRoles[role] {
		return errors.New("invalid role")
	}
	u.Role = role
	return nil
}

func (u *User) HasRole(role string) bool {
	return u.Role == role
}

func (u *User) IsAdmin() bool {
	return u.Role == "admin"
}

func (u *User) IsModerator() bool {
	return u.Role == "moderator" || u.Role == "admin"
}

func SoftDeleteUser(db *gorm.DB, id uuid.UUID) error {
	result := db.Model(&User{}).Where("id = ?", id).Update("deleted_at", time.Now())
	return result.Error
}

func RestoreUser(db *gorm.DB, id uuid.UUID) error {
	result := db.Model(&User{}).Where("id = ?", id).Update("deleted_at", nil)
	return result.Error
}

func GetDeletedUsers(db *gorm.DB, limit, offset int) ([]User, error) {
	var users []User
	result := db.Unscoped().Where("deleted_at IS NOT NULL").Limit(limit).Offset(offset).Find(&users)
	if result.Error != nil {
		return nil, result.Error
	}
	return users, nil
}

func (u *User) Lock() error {
	u.Role = "locked"
	return nil
}

func (u *User) Unlock() error {
	u.Role = "user"
	return nil
}

func (u *User) IsLocked() bool {
	return u.Role == "locked"
}

func (u *User) UpdateEmail(newEmail string) error {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(newEmail) {
		return errors.New("invalid email format")
	}
	u.Email = newEmail
	return nil
}

func (u *User) GetLastLoginTime() *time.Time {
	return u.LastLoginAt
}

func (u *User) GetAccountAge() time.Duration {
	return time.Since(u.CreatedAt)
}

func (u *User) ResetPassword() (string, error) {
	newPassword := generateRandomPassword()
	hashedPassword, err := hashPassword(newPassword)
	if err != nil {
		return "", err
	}
	u.Password = hashedPassword
	u.addPasswordToHistory(hashedPassword)
	return newPassword, nil
}

func generateRandomPassword() string {
	// Implementation of random password generation
	// This is a placeholder and should be replaced with a secure implementation
	return "RandomPassword123!"
}