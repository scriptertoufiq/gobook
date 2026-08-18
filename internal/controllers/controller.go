package controllers

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/scriptertoufiq/go-mvc/pkg/apperror"
)

// uintParam reads a positive integer route parameter, e.g. the :id in /users/:id.
func uintParam(c *gin.Context, name string) (uint, error) {
	raw := c.Param(name)

	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		return 0, apperror.BadRequest("Invalid " + name + " parameter.")
	}
	return uint(value), nil
}
