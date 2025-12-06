package handlers

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"outDrinkMeAPI/services"
)

// DocHandler struct - уверете се, че отговаря на дефиницията в main.go
type DocHandler struct {
	service *services.DocService
}

// NewDocHandler - конструкторът, който се ползва в main.go
func NewDocHandler(s *services.DocService) *DocHandler {
	return &DocHandler{service: s}
}

// ServePrivacyPolicy връща HTML страницата
func (h *DocHandler) ServePrivacyPolicy(w http.ResponseWriter, r *http.Request) {
	// Това е HTML шаблонът с вашия текст
	const privacyHtml = `
	<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>Privacy Policy - OutDrinkMe</title>
		<style>
			body {
				font-family: 'Helvetica Neue', Helvetica, Arial, sans-serif;
				line-height: 1.6;
				color: #333;
				max-width: 800px;
				margin: 0 auto;
				padding: 20px;
				background-color: #f9f9f9;
			}
			.container {
				background-color: #fff;
				padding: 40px;
				border-radius: 8px;
				box-shadow: 0 2px 4px rgba(0,0,0,0.1);
			}
			h1 { color: #2c3e50; border-bottom: 2px solid #eee; padding-bottom: 10px; }
			h2 { color: #34495e; margin-top: 30px; }
			ul { margin-bottom: 20px; }
			li { margin-bottom: 8px; }
			.date { color: #7f8c8d; font-style: italic; margin-bottom: 20px; }
			.contact { background-color: #e8f4f8; padding: 15px; border-radius: 5px; margin-top: 30px; }
			a { color: #3498db; }
		</style>
	</head>
	<body>
		<div class="container">
			<h1>Privacy Policy</h1>
			<div class="date">Last updated: October 23, 2025</div>
			
			<p>Welcome to OutDrinkMe (“we”, “our”, or “us”). This Privacy Policy explains how we collect, use, and protect your information when you use our Android app.</p>
			<p>By using OutDrinkMe, you agree to the terms of this Privacy Policy.</p>

			<h2>1. Information We Collect</h2>
			<p>We collect some personal and usage information to help make the app work better for you.</p>
			
			<h3>a. Personal Information (via Google Sign-In)</h3>
			<p>When you log in using Google OAuth, we receive the following from your Google account:</p>
			<ul>
				<li>Your first name and last name</li>
				<li>Your email address</li>
				<li>Your username (Google profile name)</li>
			</ul>

			<h3>b. Usage Data</h3>
			<p>We automatically collect some data when you use the app, such as:</p>
			<ul>
				<li>App analytics (how you use the app, time spent, etc.)</li>
				<li>Cookies or similar technologies to help us improve performance and experience</li>
			</ul>
			<p><strong>We do not collect or access your photos, contacts, camera, microphone, or location.</strong></p>

			<h2>2. How We Use Your Information</h2>
			<p>We use the information we collect to:</p>
			<ul>
				<li>Help you sign in and manage your account</li>
				<li>Improve and personalize the app experience</li>
				<li>Analyze usage and app performance</li>
				<li>Show relevant ads (via Google Ads)</li>
				<li>Maintain the security and reliability of our services</li>
			</ul>

			<h2>3. Sharing Your Information</h2>
			<p>We only share data with:</p>
			<ul>
				<li>Authentication providers (like Google, to help you log in)</li>
				<li>Database and analytics services used to store and analyze app data</li>
				<li>Advertising services such as Google Ads</li>
			</ul>
			<p>We do not sell your personal data to anyone.</p>

			<h2>4. Data Storage and Security</h2>
			<p>Your data is stored locally on your device and on our secure database servers. We use encryption and secure services to help protect your information from unauthorized access.</p>
			<p>We keep your data indefinitely unless you request deletion.</p>

			<h2>5. Your Rights</h2>
			<p>You have the right to:</p>
			<ul>
				<li>Request access to the information we have about you</li>
				<li>Ask us to delete your account and related data</li>
			</ul>
			<p>To make any requests, contact us at: <a href="mailto:martbul01@gmail.com">martbul01@gmail.com</a></p>

			<h2>6. Advertising</h2>
			<p>Our app uses Google Ads to show advertisements. Google may use cookies and similar technologies to display personalized ads based on your activity. You can manage ad personalization through your Google account settings.</p>

			<h2>7. Children’s Privacy</h2>
			<p>OutDrinkMe is not directed to children under 13. We do not knowingly collect information from children.</p>

			<h2>8. Changes to This Policy</h2>
			<p>We may update this Privacy Policy from time to time. If we make major changes, we’ll let you know by updating the date at the top of this page.</p>

			<h2>9. Contact Us</h2>
			<div class="contact">
				<p>If you have any questions or concerns about this Privacy Policy, contact us at:<br>
				<strong><a href="mailto:martbul01@gmail.com">martbul01@gmail.com</a></strong></p>
			</div>
		</div>
	</body>
	</html>
	`

	// Задаваме Header, че връщаме HTML, а не JSON
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	// Парсваме и изпълняваме шаблона
	tmpl, err := template.New("privacy").Parse(privacyHtml)
	if err != nil {
		http.Error(w, "Could not load privacy policy", http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, nil)
}

// ServeTermsOfServices връща HTML страницата за Условията за ползване
func (h *DocHandler) ServeTermsOfServices(w http.ResponseWriter, r *http.Request) {
	const termsHtml = `
	<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>Terms of Service - OutDrinkMe</title>
		<style>
			body {
				font-family: 'Helvetica Neue', Helvetica, Arial, sans-serif;
				line-height: 1.6;
				color: #333;
				max-width: 800px;
				margin: 0 auto;
				padding: 20px;
				background-color: #f9f9f9;
			}
			.container {
				background-color: #fff;
				padding: 40px;
				border-radius: 8px;
				box-shadow: 0 2px 4px rgba(0,0,0,0.1);
			}
			h1 { color: #2c3e50; border-bottom: 2px solid #eee; padding-bottom: 10px; }
			h2 { color: #34495e; margin-top: 30px; }
			ul { margin-bottom: 20px; }
			li { margin-bottom: 8px; }
			.date { color: #7f8c8d; font-style: italic; margin-bottom: 20px; }
			.contact { background-color: #e8f4f8; padding: 15px; border-radius: 5px; margin-top: 30px; }
			a { color: #3498db; }
		</style>
	</head>
	<body>
		<div class="container">
			<h1>Terms of Service</h1>
			<div class="date">Last updated: October 25, 2025</div>
			
			<p>Welcome to OutDrinkMe (“we”, “our”, or “us”). By using our Android app, you agree to these Terms of Service. Please read them carefully.</p>

			<h2>1. Eligibility</h2>
			<p>You must be 12 years or older to use OutDrinkMe. By using the app, you represent that you meet this age requirement.</p>

			<h2>2. Accounts</h2>
			<p>To use OutDrinkMe, you need to sign in with Google.</p>
			<ul>
				<li>You are responsible for maintaining the security of your account.</li>
				<li>You may add friends and see other users’ drinking stats.</li>
			</ul>
			<p>We may suspend or terminate accounts that violate these Terms.</p>

			<h2>3. User Conduct</h2>
			<p>While using OutDrinkMe, you agree to:</p>
			<ul>
				<li>Track your drinking honestly (no cheating)</li>
				<li>Respect other users’ privacy and experiences</li>
				<li>Avoid any actions that could harm the app or other users</li>
			</ul>
			<p>We reserve the right to remove content or restrict accounts that violate these rules.</p>

			<h2>4. Content and Intellectual Property</h2>
			<ul>
				<li>OutDrinkMe and its content (including designs, logos, and stats) are protected by copyright and belong to us.</li>
				<li>You may share or use app content externally, such as screenshots or stats, as long as it doesn’t violate these Terms or other users’ privacy.</li>
			</ul>

			<h2>5. Disclaimer and Limitation of Liability</h2>
			<ul>
				<li>OutDrinkMe is provided “as-is”. Errors, downtime, or data losses may occur.</li>
				<li>You agree to use the app at your own risk.</li>
				<li>We are not responsible for any consequences of your drinking, tracking errors, or use of the app.</li>
			</ul>

			<h2>6. User Responsibility</h2>
			<p>You are responsible for your actions and choices while using OutDrinkMe. By participating, you agree to use the app safely and responsibly.</p>

			<h2>7. Modifications</h2>
			<p>We may update or change these Terms of Service at any time. Major changes will be indicated by updating the date at the top. Continued use of the app after changes means you accept the new Terms.</p>

			<h2>8. Contact Us</h2>
			<div class="contact">
				<p>If you have questions about these Terms, contact us at:<br>
				<strong><a href="mailto:martbul01@gmail.com">martbul01@gmail.com</a></strong></p>
			</div>
		</div>
	</body>
	</html>
	`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	tmpl, err := template.New("terms").Parse(termsHtml)
	if err != nil {
		http.Error(w, "Could not load terms of service", http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, nil)
}




func (h *DocHandler) GetAppMinVersion(w http.ResponseWriter, r *http.Request) {
	appAndroidMinVersion := os.Getenv("ANDROID_MIN_VERSION")
	if appAndroidMinVersion == "" {
		log.Fatal("appAndroidMinVersion environment variable is not set")
	}

	appIOSMinVersion := os.Getenv("IOS_MIN_VERSION")
	if appIOSMinVersion == "" {
		log.Fatal("appIOSMinVersion environment variable is not set")
	}

	type MinVersion struct {
		MinAndroidVersion string `json:"min_android_version_code"`
		MinIOSVersion     string `json:"min_ios_version_code"`
		UpdateMessage     string `json:"update_message"`
	}

	minVers := &MinVersion{
		MinAndroidVersion: appAndroidMinVersion,
		MinIOSVersion:     appIOSMinVersion,
		UpdateMessage:     "An important update is available! Please update to continue using the app. This update includes critical server compatibility changes",
	}

	
	respondWithJSON(w, http.StatusOK, minVers)
}
