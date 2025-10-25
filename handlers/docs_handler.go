package handlers

import (
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
