package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hekemen/automata/internal/domain/context"
)

type ContextUserHandler struct {
	userRepo    context.UserRepository
	contextRepo context.Repository
}

func NewContextUserHandler(userRepo context.UserRepository, contextRepo context.Repository) *ContextUserHandler {
	return &ContextUserHandler{userRepo: userRepo, contextRepo: contextRepo}
}

type AddContextUserRequest struct {
	UserID string `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"required,oneof=owner admin member"`
}

func (h *ContextUserHandler) List(c *gin.Context) {
	contextID := c.Param("id")
	if _, err := uuid.Parse(contextID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context ID"})
		return
	}
	members, err := h.userRepo.ListMembers(contextID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"members": members})
}

func (h *ContextUserHandler) Create(c *gin.Context) {
	contextID := c.Param("id")
	if _, err := uuid.Parse(contextID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context ID"})
		return
	}
	var req AddContextUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	uc := &context.UserContext{
		UserID:    req.UserID,
		ContextID: contextID,
		Role:      req.Role,
	}
	if err := h.userRepo.CreateMembership(uc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"user_id":    uc.UserID,
		"context_id": uc.ContextID,
		"role":       uc.Role,
	})
}

func (h *ContextUserHandler) Delete(c *gin.Context) {
	contextID := c.Param("id")
	userID := c.Param("userId")
	if _, err := uuid.Parse(contextID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context ID"})
		return
	}
	if _, err := uuid.Parse(userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}
	if err := h.userRepo.DeleteMembership(userID, contextID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "membership not found"})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}
