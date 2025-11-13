package constants

import "errors"

type AppName string
type Status string
type USERID string
type RequestID string
type OtpTemplateId string

const UserID USERID = "userID"
const REQUEST_ID RequestID = "requestID"
const LOGGED_IN_USER = "loggedInUser"

const (
	ApiSuccess Status = "success"
	ApiFailure Status = "failure"
)

const (
	Reset OtpTemplateId = "reset"
	Login OtpTemplateId = "login"
)
const API_TOKEN string = "Authorization"

const (
	TIMEZONE = "Asia/Kolkata"
)

var (
	ErrorInvalidOTP     error = errors.New("invalid OTP")
	ErrorMaxAttempts    error = errors.New("max attempts reached")
	ErrorOtpExpired     error = errors.New("otp expired")
	ErrorInvalidRequest error = errors.New("invalid request")
)

const MaxFileSize = 1024 * 1024
