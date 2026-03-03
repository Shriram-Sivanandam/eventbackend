package email

import (
	"fmt"
	"os"

	"github.com/resend/resend-go/v2"
)

func SendOTP(toEmail string, otp string) error {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("RESEND_API_KEY not set")
	}

	client := resend.NewClient(apiKey)

	html := buildOTPEmail(otp)

	params := &resend.SendEmailRequest{
		From:    "Spotlight <onboarding@resend.dev>", 
		To:      []string{toEmail},
		Subject: fmt.Sprintf("Your OTP is %s", otp),
		Html:    html,
	}

	_, err := client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func buildOTPEmail(otp string) string {
	return fmt.Sprintf(`
		<div style="font-family: sans-serif; max-width: 480px; margin: 0 auto; padding: 32px;">
			<h2 style="color: #1A0A00; margin-bottom: 8px;">Your login code</h2>
			<p style="color: #5C4F42; margin-bottom: 24px;">
				Use the code below to sign in. It expires in <strong>5 minutes</strong>.
			</p>
			<div style="
				background: #FAF7F2;
				border: 1.5px solid #EDE8DF;
				border-radius: 12px;
				padding: 24px;
				text-align: center;
				letter-spacing: 8px;
				font-size: 32px;
				font-weight: 800;
				color: #FF6B35;
			">
				%s
			</div>
			<p style="color: #C4BAB0; font-size: 12px; margin-top: 24px;">
				If you didn't request this, you can safely ignore this email.
			</p>
		</div>
	`, otp)
}