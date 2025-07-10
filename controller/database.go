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
// @Description Update an existing record in a specific table (condition+update).
// @Tags Database
// @Accept json
// @Produce json
// @Param name path string true "Table Name"
// @Param body body map[string]interface{} true "Update request {condition:{},update:{}}"
// @Success 200 {object} common.Response
// @Failure 400 {object} common.Response
// @Failure 500 {object} common.Response
// @Router /api/database/tables/{name} [put]
func UpdateTableData(c *gin.Context) {
	tableName := c.Param("name")
	if tableName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Table name is required"})
		return
	}

	var req struct {
		Condition map[string]interface{} `json:"condition"`
		Update    map[string]interface{} `json:"update"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid request body: " + err.Error()})
		return
	}
	if len(req.Condition) == 0 || len(req.Update) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Both condition and update are required"})
		return
	}

	where := ""
	args := []interface{}{}
	for k, v := range req.Condition {
		if where != "" {
			where += " AND "
		}
		where += fmt.Sprintf("`%s` = ?", k)
		args = append(args, v)
	}

	tx := model.DB.Table(tableName).Where(where, args...).Updates(req.Update)
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to update record: " + tx.Error.Error(), "rows": tx.RowsAffected})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Record updated successfully", "rows": tx.RowsAffected})
}

type BulkUpdateItem struct {
	Condition map[string]interface{} `json:"condition"`
	Update    map[string]interface{} `json:"update"`
}
type BulkUpdateRequest struct {
	Items []BulkUpdateItem `json:"items"`
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

	if len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Items are required for bulk update"})
		return
	}

	results := make([]map[string]interface{}, 0, len(req.Items))
	for _, item := range req.Items {
		res := map[string]interface{}{
			"ok":    true,
			"error": "",
		}
		// 记录id（如有）
		if idVal, ok := item.Condition["id"]; ok {
			res["id"] = idVal
		}
		// 构造where
		where := ""
		args := []interface{}{}
		for k, v := range item.Condition {
			if where != "" {
				where += " AND "
			}
			where += fmt.Sprintf("`%s` = ?", k)
			args = append(args, v)
		}
		tx := model.DB.Table(tableName).Where(where, args...).Updates(item.Update)
		if tx.Error != nil {
			res["ok"] = false
			res["error"] = tx.Error.Error()
		}
		results = append(results, res)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Bulk update finished",
		"results": results,
	})
}

type BulkDeleteRequest struct {
	Conditions []map[string]interface{} `json:"conditions"`
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

	if len(req.Conditions) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Conditions are required for bulk delete"})
		return
	}

	results := make([]map[string]interface{}, 0, len(req.Conditions))
	for _, condition := range req.Conditions {
		res := map[string]interface{}{
			"ok":    true,
			"error": "",
		}
		// Record the condition (e.g., 'id' if present) for the response
		if idVal, ok := condition["id"]; ok {
			res["id"] = idVal
		} else {
			// If no 'id', use the full condition map as identifier for response
			res["condition"] = condition
		}

		where := ""
		args := []interface{}{}
		for k, v := range condition {
			if where != "" {
				where += " AND "
			}
			where += fmt.Sprintf("`%s` = ?", k)
			args = append(args, v)
		}
		err := model.DB.Table(tableName).Where(where, args...).Delete(nil).Error
		if err != nil {
			res["ok"] = false
			res["error"] = err.Error()
		}
		results = append(results, res)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Bulk delete finished",
		"results": results,
	})
}

// DeleteTableData godoc
// @Summary Delete table data
// @Description Delete a record from a specific table.
// @Tags Database
// @Accept json
// @Produce json
// @Param name path string true "Table Name"
// @Param data body map[string]interface{} true "Data to delete (all fields used as condition)"
// @Success 200 {object} common.Response
// @Failure 400 {object} common.Response
// @Failure 500 {object} common.Response
// @Router /api/database/tables/{name} [delete]
func DeleteTableData(c *gin.Context) {
	tableName := c.Param("name")
	if tableName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Table name is required",
		})
		return
	}

	var data map[string]interface{}
	if errBind := c.ShouldBindJSON(&data); errBind != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request body: " + errBind.Error(),
		})
		return
	}

	where := ""
	args := []interface{}{}
	for k, v := range data {
		if where != "" {
			where += " AND "
		}
		where += fmt.Sprintf("`%s` = ?", k)
		args = append(args, v)
	}
	tx := model.DB.Table(tableName).Where(where, args...).Delete(nil)
	err := tx.Error
	rows := tx.RowsAffected

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete record: " + err.Error(),
			"rows":    rows,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Record deleted successfully",
		"rows":    rows,
	})
}
