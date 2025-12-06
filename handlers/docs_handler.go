package handlers

import (
	"log"
	"net/http"
	"os"
	"outDrinkMeAPI/services"
)

type DocHandler struct {
	docService *services.DocService
}

func NewDocHandler(docService *services.DocService) *DocHandler {
	return &DocHandler{
		docService: docService,
	}
}

func (h *DocHandler) ServePrivacyPolicy(w http.ResponseWriter, r *http.Request) {
	filePath := "doc/OutDrinkMe_PrivacyPolicy.docx"

	// Read the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "Privacy policy not found", http.StatusNotFound)
		return
	}

	// Set headers for DOCX download
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	w.Header().Set("Content-Disposition", "inline; filename=OutDrinkMe_PrivacyPolicy.docx")

	w.WriteHeader(http.StatusOK)
	w.Write(content)
}

func (h *DocHandler) ServeTermsOfServices(w http.ResponseWriter, r *http.Request) {
	filePath := "doc/OutDrinkMe_TermsOfService.docx"

	// Read the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "Terms of services not found", http.StatusNotFound)
		return
	}

	// Set headers for DOCX download
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	w.Header().Set("Content-Disposition", "inline; filename=OutDrinkMe_TermsOfService.docx")

	w.WriteHeader(http.StatusOK)
	w.Write(content)
}

func (h *DocHandler) GetAppMinVersion(w http.ResponseWriter, r *http.Request) {
	appAndroidMinVersion := os.Getenv("ANDROID_MIN_VERSION")
	if appAndroidMinVersion == "" {
		log.Fatal("appAndroidMinVersion environment variable is not set")
	}

	appIOSMinVersion := os.Getenv("	")
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
