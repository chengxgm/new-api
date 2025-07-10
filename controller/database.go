package controller

import (
	"fmt"
	"net/http"
	"one-api/common"
	"one-api/model"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// GetTables godoc
// @Summary Get all table names
// @Description Get a list of all table names in the database.
// @Tags Database
// @Accept json
// @Produce json
// @Success 200 {object} common.Response{data=[]string}
// @Failure 500 {object} common.Response
// @Router /api/database/tables [get]
func GetTables(c *gin.Context) {
	var tables []string
	var err error

	if common.UsingSQLite {
		// For SQLite, query sqlite_master table
		type SQLiteMaster struct {
			Name string `gorm:"column:name"`
			Type string `gorm:"column:type"`
		}
		var sqliteMasters []SQLiteMaster
		err = model.DB.Raw("SELECT name, type FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE 'gorm_%'").Scan(&sqliteMasters).Error
		if err == nil {
			for _, sm := range sqliteMasters {
				tables = append(tables, sm.Name)
			}
		}
	} else if common.UsingMySQL {
		err = model.DB.Raw("SHOW TABLES").Scan(&tables).Error
	} else if common.UsingPostgreSQL {
		err = model.DB.Raw("SELECT tablename FROM pg_tables WHERE schemaname = 'public'").Scan(&tables).Error
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unsupported database type",
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to get table names: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Success",
		"data":    tables,
	})
}

// GetTableInfo godoc
// @Summary Get table information
// @Description Get the schema (columns) of a specific table.
// @Tags Database
// @Accept json
// @Produce json
// @Param name path string true "Table Name"
// @Success 200 {object} common.Response{data=[]map[string]interface{}}
// @Failure 400 {object} common.Response
// @Failure 500 {object} common.Response
// @Router /api/database/tables/{name}/info [get]
func GetTableInfo(c *gin.Context) {
	tableName := c.Param("name")
	if tableName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Table name is required",
		})
		return
	}

	var columns []map[string]interface{}
	var err error

	if common.UsingSQLite {
		// For SQLite, use PRAGMA table_info
		err = model.DB.Raw(fmt.Sprintf("PRAGMA table_info(%s)", tableName)).Scan(&columns).Error
	} else if common.UsingMySQL {
		err = model.DB.Raw(fmt.Sprintf("DESCRIBE `%s`", tableName)).Scan(&columns).Error
	} else if common.UsingPostgreSQL {
		err = model.DB.Raw(`
			SELECT column_name, data_type, is_nullable, column_default
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = ?
			ORDER BY ordinal_position;
		`, tableName).Scan(&columns).Error
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Unsupported database type",
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to get table info: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Success",
		"data":    columns,
	})
}

// GetTableData godoc
// @Summary Get table data
// @Description Get data from a specific table with pagination.
// @Tags Database
// @Accept json
// @Produce json
// @Param name path string true "Table Name"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Success 200 {object} common.Response{data=[]map[string]interface{}}
// @Failure 400 {object} common.Response
// @Failure 500 {object} common.Response
// @Router /api/database/tables/{name} [get]
func GetTableData(c *gin.Context) {
	tableName := c.Param("name")
	if tableName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Table name is required",
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	var results []map[string]interface{}
	var total int64

	// Count total records
	err := model.DB.Table(tableName).Count(&total).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to count records: " + err.Error(),
		})
		return
	}

	// Fetch paginated data
	err = model.DB.Table(tableName).Offset((page - 1) * pageSize).Limit(pageSize).Find(&results).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to get table data: " + err.Error(),
		})
		return
	}

	// Convert all time.Time fields to RFC3339 strings for consistency
	for i := range results {
		for k, v := range results[i] {
			switch tv := v.(type) {
			case time.Time:
				results[i][k] = tv.UTC().Format(time.RFC3339)
			case *time.Time:
				if tv != nil {
					results[i][k] = tv.UTC().Format(time.RFC3339)
				} else {
					results[i][k] = nil
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Success",
		"data":    results,
		"total":   total,
	})
}

// CreateTableData godoc
// @Summary Create table data
// @Description Create a new record in a specific table.
// @Tags Database
// @Accept json
// @Produce json
// @Param name path string true "Table Name"
// @Param data body map[string]interface{} true "Data to create"
// @Success 200 {object} common.Response
// @Failure 400 {object} common.Response
// @Failure 500 {object} common.Response
// @Router /api/database/tables/{name} [post]
func CreateTableData(c *gin.Context) {
	tableName := c.Param("name")
	if tableName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Table name is required",
		})
		return
	}

	var data map[string]interface{}
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request body: " + err.Error(),
		})
		return
	}

	err := model.DB.Table(tableName).Create(&data).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create record: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Record created successfully",
	})
}

// UpdateTableData godoc
// @Summary Update table data
// @Description Update an existing record in a specific table.
// @Tags Database
// @Accept json
// @Produce json
// @Param name path string true "Table Name"
// @Param id path int true "Record ID"
// @Param data body map[string]interface{} true "Data to update"
// @Success 200 {object} common.Response
// @Failure 400 {object} common.Response
// @Failure 500 {object} common.Response
// @Router /api/database/tables/{name}/{id} [put]
func UpdateTableData(c *gin.Context) {
	tableName := c.Param("name")
	id := c.Param("id")
	if tableName == "" || id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Table name and ID are required",
		})
		return
	}

	var data map[string]interface{}
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request body: " + err.Error(),
		})
		return
	}

	// GORM will automatically use the primary key for updates if it's present in the map.
	// Assuming 'id' is the primary key column name.
	// For generic tables, we need to be careful about the primary key name.
	// For simplicity, we'll assume 'id' is the primary key column name for now.
	// A more robust solution would involve querying table info to find the actual primary key.
	err := model.DB.Table(tableName).Where("id = ?", id).Updates(data).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update record: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Record updated successfully",
	})
}

type BulkUpdateRequest struct {
	IDs    []int                  `json:"ids"`
	Update map[string]interface{} `json:"update"`
}

// BulkUpdateTableData godoc
// @Summary Bulk update table data
// @Description Bulk update records in a specific table.
// @Tags Database
// @Accept json
// @Produce json
// @Param name path string true "Table Name"
// @Param body body BulkUpdateRequest true "Bulk update request"
// @Success 200 {object} common.Response
// @Failure 400 {object} common.Response
// @Failure 500 {object} common.Response
// @Router /api/database/tables/{name}/bulk-update [put]
func BulkUpdateTableData(c *gin.Context) {
	tableName := c.Param("name")
	if tableName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Table name is required"})
		return
	}

	var req BulkUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid request body: " + err.Error()})
		return
	}

	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "IDs are required for bulk update"})
		return
	}

	if len(req.Update) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Update data is required"})
		return
	}

	err := model.DB.Table(tableName).Where("id IN (?)", req.IDs).Updates(req.Update).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to bulk update records: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Records updated successfully"})
}

type BulkDeleteRequest struct {
	PK  string `json:"pk"`
	IDs []int  `json:"ids"`
}

// BulkDeleteTableData godoc
// @Summary Bulk delete table data
// @Description Bulk delete records in a specific table.
// @Tags Database
// @Accept json
// @Produce json
// @Param name path string true "Table Name"
// @Param body body BulkDeleteRequest true "Bulk delete request"
// @Success 200 {object} common.Response
// @Failure 400 {object} common.Response
// @Failure 500 {object} common.Response
// @Router /api/database/tables/{name}/bulk-delete [delete]
func BulkDeleteTableData(c *gin.Context) {
	tableName := c.Param("name")
	if tableName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Table name is required"})
		return
	}

	var req BulkDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid request body: " + err.Error()})
		return
	}

	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "IDs are required for bulk delete"})
		return
	}

	pk := req.PK
	if pk == "" {
		pk = "id"
	}
	err := model.DB.Table(tableName).Where(fmt.Sprintf("%s IN (?)", pk), req.IDs).Delete(nil).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to bulk delete records: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Records deleted successfully"})
}

// DeleteTableData godoc
// @Summary Delete table data
// @Description Delete a record from a specific table.
// @Tags Database
// @Accept json
// @Produce json
// @Param name path string true "Table Name"
// @Param id path int true "Record ID"
// @Success 200 {object} common.Response
// @Failure 400 {object} common.Response
// @Failure 500 {object} common.Response
// @Router /api/database/tables/{name}/{id} [delete]
func DeleteTableData(c *gin.Context) {
	tableName := c.Param("name")
	id := c.Param("id")
	if tableName == "" || id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Table name and ID are required",
		})
		return
	}

	// For generic tables, we need to be careful about the primary key name.
	// Assuming 'id' is the primary key column name for now.
	err := model.DB.Table(tableName).Where("id = ?", id).Delete(nil).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete record: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Record deleted successfully",
	})
}
