package main

import (
	"context"
	"net/http"
)

func forgotPasswordFlow(database Database) challengeFlow {
	return challengeFlow{
		table:             passwordResetChallenges,
		userMustExist:     true,
		userFailureStatus: http.StatusNotFound,
		userFailureMsg:    "User not found",
		subject:           "Trading simulation password reset code",
		instruction:       "Your password reset code is:",
		sendEmailErrMsg:   "Could not send password reset email",
		invalidDataMsg:    "Invalid password reset data",
		complete: func(ctx context.Context, challengeID string, passwordHash string, r *http.Request) error {
			return database.CompletePasswordReset(ctx, challengeID, passwordHash)
		},
		completeErrMsg: "Could not reset password",
		successMsg:     "Password reset complete",
	}
}

func forgotPasswordStart(database Database) http.HandlerFunc {
	return challengeStart(database, forgotPasswordFlow(database))
}

func forgotPasswordComplete(database Database) http.HandlerFunc {
	return challengeComplete(database, forgotPasswordFlow(database))
}
