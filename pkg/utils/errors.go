package utils

import (
	"fmt"
	"runtime"
	"strings"
)

// CustomError represents a custom error type with additional context
type CustomError struct {
	Message string
	Code    int
	File    string
	Line    int
	Stack   string
	Context map[string]interface{}
}

// Error implements the error interface for CustomError
func (e *CustomError) Error() string {
	contextStr := formatContext(e.Context)
	return fmt.Sprintf("Error %d: %s (at %s:%d)\nContext: %s\n%s", e.Code, e.Message, e.File, e.Line, contextStr, e.Stack)
}

// NewError creates a new CustomError with the given message and code
func NewError(message string, code int) *CustomError {
	return newCustomError(message, code, 2, nil)
}

// WrapError wraps an existing error with additional context
func WrapError(err error, message string, code ...int) *CustomError {
	errorCode := 500
	if len(code) > 0 {
		errorCode = code[0]
	}
	return newCustomError(fmt.Sprintf("%s: %v", message, err), errorCode, 2, nil)
}

func newCustomError(message string, code, skip int, context map[string]interface{}) *CustomError {
	_, file, line, _ := runtime.Caller(skip)
	stack := captureStack(skip + 1)
	return &CustomError{
		Message: message,
		Code:    code,
		File:    file,
		Line:    line,
		Stack:   stack,
		Context: context,
	}
}

// IsCustomError checks if an error is of type CustomError
func IsCustomError(err error) bool {
	_, ok := err.(*CustomError)
	return ok
}

// GetErrorCode returns the error code if it's a CustomError, otherwise returns -1
func GetErrorCode(err error) (int, bool) {
	if customErr, ok := err.(*CustomError); ok {
		return customErr.Code, true
	}
	return -1, false
}

// NotFoundError represents a resource not found error
type NotFoundError struct {
	CustomError
}

// NewNotFoundError creates a new NotFoundError
func NewNotFoundError(message string, context ...map[string]interface{}) *NotFoundError {
	ctx := make(map[string]interface{})
	if len(context) > 0 {
		ctx = context[0]
	}
	return &NotFoundError{
		CustomError: *newCustomError(message, 404, 2, ctx),
	}
}

// BadRequestError represents a bad request error
type BadRequestError struct {
	CustomError
}

// NewBadRequestError creates a new BadRequestError
func NewBadRequestError(message string, context ...map[string]interface{}) *BadRequestError {
	ctx := make(map[string]interface{})
	if len(context) > 0 {
		ctx = context[0]
	}
	return &BadRequestError{
		CustomError: *newCustomError(message, 400, 2, ctx),
	}
}

// UnauthorizedError represents an unauthorized access error
type UnauthorizedError struct {
	CustomError
}

// NewUnauthorizedError creates a new UnauthorizedError
func NewUnauthorizedError(message string, context ...map[string]interface{}) *UnauthorizedError {
	ctx := make(map[string]interface{})
	if len(context) > 0 {
		ctx = context[0]
	}
	return &UnauthorizedError{
		CustomError: *newCustomError(message, 401, 2, ctx),
	}
}

// InternalServerError represents an internal server error
type InternalServerError struct {
	CustomError
}

// NewInternalServerError creates a new InternalServerError
func NewInternalServerError(message string, context ...map[string]interface{}) *InternalServerError {
	ctx := make(map[string]interface{})
	if len(context) > 0 {
		ctx = context[0]
	}
	return &InternalServerError{
		CustomError: *newCustomError(message, 500, 2, ctx),
	}
}

func captureStack(skip int) string {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(skip, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])

	var stackTrace strings.Builder
	for {
		frame, more := frames.Next()
		if !more {
			break
		}
		stackTrace.WriteString(fmt.Sprintf("%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line))
	}
	return stackTrace.String()
}

// IsErrorType checks if an error is of a specific custom error type
func IsErrorType[T error](err error) bool {
	_, ok := err.(T)
	return ok
}

// GetErrorMessage returns a user-friendly error message based on the error type
func GetErrorMessage(err error) string {
	switch err.(type) {
	case *NotFoundError:
		return "The requested resource was not found."
	case *BadRequestError:
		return "The request was invalid or cannot be served."
	case *UnauthorizedError:
		return "Authentication is required to access this resource."
	case *InternalServerError:
		return "An unexpected error occurred. Please try again later."
	default:
		return "An error occurred."
	}
}

// GetErrorDetails returns detailed error information for debugging purposes
func GetErrorDetails(err error) string {
	if customErr, ok := err.(*CustomError); ok {
		contextStr := formatContext(customErr.Context)
		return fmt.Sprintf("Error Code: %d\nMessage: %s\nFile: %s\nLine: %d\nContext: %s\nStack Trace:\n%s",
			customErr.Code, customErr.Message, customErr.File, customErr.Line, contextStr, customErr.Stack)
	}
	return err.Error()
}

// NewErrorWithContext creates a new CustomError with additional context
func NewErrorWithContext(message string, code int, context map[string]interface{}) *CustomError {
	return newCustomError(message, code, 2, context)
}

func formatContext(context map[string]interface{}) string {
	var builder strings.Builder
	for k, v := range context {
		builder.WriteString(fmt.Sprintf("%s: %v\n", k, v))
	}
	return builder.String()
}