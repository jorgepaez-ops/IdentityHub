package api

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/jorgepaez/identity-hub/internal/store"
)

// organizationDirectory is the read side of the company model (RF-022, ADR 0015): the companies the
// console can pick from and the applications a user can launch.
type organizationDirectory interface {
	ListOrganizations(context.Context) ([]store.Organization, error)
	ListUserApplications(context.Context, uuid.UUID) ([]store.UserApplication, error)
}

// requiredOrganizationID reads the mandatory organizationId query value after the admin wrapper has
// authenticated the caller. It writes the 400 problem and reports false when it is absent or not a UUID.
func requiredOrganizationID(w http.ResponseWriter, value *OrganizationId) (uuid.UUID, bool) {
	if value != nil {
		if id, err := uuid.Parse(*value); err == nil && id != uuid.Nil {
			return id, true
		}
	}
	writeValidationProblem(w, "organizationId", "must be a company identifier")
	return uuid.Nil, false
}

// ListOrganizations is GET /api/v1/admin/organizations, restricted to administrators.
func (s *Server) ListOrganizations(w http.ResponseWriter, r *http.Request) {
	s.adminOnly(s.listOrganizations).ServeHTTP(w, r)
}

// ListMyApplications is GET /api/v1/me/applications, open to any authenticated Hub user.
func (s *Server) ListMyApplications(w http.ResponseWriter, r *http.Request) {
	RequireAuth(s.tokens)(http.HandlerFunc(s.listMyApplications)).ServeHTTP(w, r)
}

func (s *Server) listOrganizations(w http.ResponseWriter, r *http.Request) {
	if s.organizations == nil {
		writeProblem(w, http.StatusServiceUnavailable, "organizations-unavailable", titleServiceUnavailable, "Organizations are temporarily unavailable.")
		return
	}
	organizations, err := s.organizations.ListOrganizations(r.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "organizations-load-failed", titleInternalServerError, "Organizations could not be loaded.")
		return
	}
	result := make([]Organization, 0, len(organizations))
	for _, organization := range organizations {
		result = append(result, Organization{Id: openapi_types.UUID(organization.ID), Slug: organization.Slug, Name: organization.Name, Operator: organization.ID == store.HubOrganizationID})
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) listMyApplications(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeUnauthorized(w)
		return
	}
	if s.organizations == nil {
		writeProblem(w, http.StatusServiceUnavailable, "applications-unavailable", titleServiceUnavailable, "Applications are temporarily unavailable.")
		return
	}
	applications, err := s.organizations.ListUserApplications(r.Context(), userID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "applications-load-failed", titleInternalServerError, "Applications could not be loaded.")
		return
	}
	result := make([]UserApplication, 0, len(applications))
	for _, application := range applications {
		result = append(result, UserApplication{ClientId: application.ClientID, Name: application.Name, LaunchUrl: application.LaunchURL})
	}
	writeJSON(w, http.StatusOK, result)
}
