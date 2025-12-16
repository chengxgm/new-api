package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

type mgrColumnInfo struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	NotNull      bool    `json:"notnull"`
	DefaultValue *string `json:"default_value,omitempty"`
	PK           bool    `json:"pk"`
}

type mgrSchemaCache struct {
	byInputName map[string]*schema.Field // accepts both json tag name and DB column name
}

var (
	channelMgrSchemaOnce sync.Once
	channelMgrSchema     mgrSchemaCache
	channelMgrSchemaErr  error
)

func getChannelMgrSchema() (mgrSchemaCache, error) {
	channelMgrSchemaOnce.Do(func() {
		stmt := &gorm.Statement{DB: model.DB}
		if err := stmt.Parse(&model.Channel{}); err != nil {
			channelMgrSchemaErr = err
			return
		}
		m := make(map[string]*schema.Field, len(stmt.Schema.Fields)*2)
		for _, f := range stmt.Schema.Fields {
			// Allow DB column name access
			if f.DBName != "" {
				m[f.DBName] = f
			}
			// Allow JSON tag access
			j := strings.TrimSpace(f.Tag.Get("json"))
			if j == "" {
				continue
			}
			j = strings.Split(j, ",")[0]
			if j == "" || j == "-" {
				continue
			}
			m[j] = f
		}
		channelMgrSchema = mgrSchemaCache{byInputName: m}
	})
	return channelMgrSchema, channelMgrSchemaErr
}

func mgrWriteJSON(c *gin.Context, status int, data interface{}) {
	c.JSON(status, data)
}

func mgrWriteError(c *gin.Context, status int, err error) {
	mgrWriteJSON(c, status, gin.H{"error": err.Error()})
}

func mgrWriteErrorMsg(c *gin.Context, status int, msg string) {
	mgrWriteJSON(c, status, gin.H{"error": msg})
}

func mgrToInt64(v interface{}) int64 {
	switch t := v.(type) {
	case nil:
		return 0
	case int:
		return int64(t)
	case int8:
		return int64(t)
	case int16:
		return int64(t)
	case int32:
		return int64(t)
	case int64:
		return t
	case uint:
		return int64(t)
	case uint8:
		return int64(t)
	case uint16:
		return int64(t)
	case uint32:
		return int64(t)
	case uint64:
		if t > ^uint64(0)>>1 {
			return 0
		}
		return int64(t)
	case float32:
		return int64(t)
	case float64:
		return int64(t)
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0
		}
		n, _ := strconv.ParseInt(s, 10, 64)
		return n
	case json.Number:
		n, _ := t.Int64()
		return n
	default:
		return 0
	}
}

func mgrConvertValue(fieldType reflect.Type, v interface{}) (interface{}, error) {
	if fieldType.Kind() == reflect.Pointer {
		if v == nil {
			return nil, nil
		}
		return mgrConvertValue(fieldType.Elem(), v)
	}

	// Special case: channel_info (custom struct stored as JSON)
	if fieldType == reflect.TypeOf(model.ChannelInfo{}) {
		switch t := v.(type) {
		case nil:
			return model.ChannelInfo{}, nil
		case string:
			s := strings.TrimSpace(t)
			if s == "" {
				return model.ChannelInfo{}, nil
			}
			var ci model.ChannelInfo
			if err := json.Unmarshal([]byte(s), &ci); err != nil {
				return nil, err
			}
			return ci, nil
		default:
			b, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			var ci model.ChannelInfo
			if err := json.Unmarshal(b, &ci); err != nil {
				return nil, err
			}
			return ci, nil
		}
	}

	switch fieldType.Kind() {
	case reflect.String:
		switch t := v.(type) {
		case nil:
			return "", nil
		case string:
			return t, nil
		case []byte:
			return string(t), nil
		default:
			// if a structured value comes in (map/array), keep behavior predictable: JSON stringify
			b, err := json.Marshal(t)
			if err != nil {
				return fmt.Sprint(t), nil
			}
			return string(b), nil
		}
	case reflect.Bool:
		switch t := v.(type) {
		case bool:
			return t, nil
		case string:
			s := strings.TrimSpace(strings.ToLower(t))
			return s == "true" || s == "1" || s == "yes" || s == "y", nil
		case float64:
			return t != 0, nil
		default:
			return false, nil
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch t := v.(type) {
		case int:
			return int64(t), nil
		case int64:
			return t, nil
		case float64:
			return int64(t), nil
		case json.Number:
			return t.Int64()
		case string:
			s := strings.TrimSpace(t)
			if s == "" {
				return int64(0), nil
			}
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return nil, err
			}
			return n, nil
		default:
			return int64(0), nil
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		switch t := v.(type) {
		case uint:
			return uint64(t), nil
		case uint64:
			return t, nil
		case int:
			if t < 0 {
				return uint64(0), nil
			}
			return uint64(t), nil
		case int64:
			if t < 0 {
				return uint64(0), nil
			}
			return uint64(t), nil
		case float64:
			if t < 0 {
				return uint64(0), nil
			}
			return uint64(t), nil
		case string:
			s := strings.TrimSpace(t)
			if s == "" {
				return uint64(0), nil
			}
			n, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				return nil, err
			}
			return n, nil
		default:
			return uint64(0), nil
		}
	case reflect.Float32, reflect.Float64:
		switch t := v.(type) {
		case float32:
			return float64(t), nil
		case float64:
			return t, nil
		case json.Number:
			return t.Float64()
		case string:
			s := strings.TrimSpace(t)
			if s == "" {
				return float64(0), nil
			}
			f, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return nil, err
			}
			return f, nil
		default:
			return float64(0), nil
		}
	default:
		// fallback: keep raw
		return v, nil
	}
}

func mgrSetStructField(dst reflect.Value, f *schema.Field, v interface{}) error {
	fv := dst.FieldByName(f.Name)
	if !fv.IsValid() || !fv.CanSet() {
		return nil
	}
	if fv.Kind() == reflect.Pointer {
		if v == nil {
			fv.Set(reflect.Zero(fv.Type()))
			return nil
		}
		converted, err := mgrConvertValue(fv.Type().Elem(), v)
		if err != nil {
			return err
		}
		elem := reflect.New(fv.Type().Elem())
		cv := reflect.ValueOf(converted)
		if cv.IsValid() {
			if cv.Type().AssignableTo(fv.Type().Elem()) {
				elem.Elem().Set(cv)
			} else if cv.Type().ConvertibleTo(fv.Type().Elem()) {
				elem.Elem().Set(cv.Convert(fv.Type().Elem()))
			} else {
				// best-effort: for numeric conversions we often return int64/uint64/float64
				switch fv.Type().Elem().Kind() {
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					if n, ok := converted.(int64); ok {
						elem.Elem().SetInt(n)
					} else if n, ok := converted.(uint64); ok {
						elem.Elem().SetInt(int64(n))
					} else if n, ok := converted.(float64); ok {
						elem.Elem().SetInt(int64(n))
					}
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					if n, ok := converted.(uint64); ok {
						elem.Elem().SetUint(n)
					} else if n, ok := converted.(int64); ok && n >= 0 {
						elem.Elem().SetUint(uint64(n))
					} else if n, ok := converted.(float64); ok && n >= 0 {
						elem.Elem().SetUint(uint64(n))
					}
				case reflect.Float32, reflect.Float64:
					if n, ok := converted.(float64); ok {
						elem.Elem().SetFloat(n)
					} else if n, ok := converted.(int64); ok {
						elem.Elem().SetFloat(float64(n))
					} else if n, ok := converted.(uint64); ok {
						elem.Elem().SetFloat(float64(n))
					}
				case reflect.String:
					if s, ok := converted.(string); ok {
						elem.Elem().SetString(s)
					} else {
						elem.Elem().SetString(fmt.Sprint(converted))
					}
				}
			}
		}
		fv.Set(elem)
		return nil
	}

	converted, err := mgrConvertValue(fv.Type(), v)
	if err != nil {
		return err
	}
	cv := reflect.ValueOf(converted)
	if !cv.IsValid() {
		fv.Set(reflect.Zero(fv.Type()))
		return nil
	}
	if cv.Type().AssignableTo(fv.Type()) {
		fv.Set(cv)
		return nil
	}
	if cv.Type().ConvertibleTo(fv.Type()) {
		fv.Set(cv.Convert(fv.Type()))
		return nil
	}
	switch fv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if n, ok := converted.(int64); ok {
			fv.SetInt(n)
		} else if n, ok := converted.(uint64); ok {
			fv.SetInt(int64(n))
		} else if n, ok := converted.(float64); ok {
			fv.SetInt(int64(n))
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if n, ok := converted.(uint64); ok {
			fv.SetUint(n)
		} else if n, ok := converted.(int64); ok && n >= 0 {
			fv.SetUint(uint64(n))
		} else if n, ok := converted.(float64); ok && n >= 0 {
			fv.SetUint(uint64(n))
		}
	case reflect.Float32, reflect.Float64:
		if n, ok := converted.(float64); ok {
			fv.SetFloat(n)
		} else if n, ok := converted.(int64); ok {
			fv.SetFloat(float64(n))
		} else if n, ok := converted.(uint64); ok {
			fv.SetFloat(float64(n))
		}
	case reflect.String:
		if s, ok := converted.(string); ok {
			fv.SetString(s)
		} else {
			fv.SetString(fmt.Sprint(converted))
		}
	}
	return nil
}

func mgrBuildUpdateMap(values map[string]interface{}, createMode bool) (map[string]interface{}, *int, error) {
	if values == nil {
		return nil, nil, nil
	}
	sc, err := getChannelMgrSchema()
	if err != nil {
		return nil, nil, err
	}
	updates := make(map[string]interface{}, len(values))
	var status *int

	for k, v := range values {
		f := sc.byInputName[k]
		if f == nil {
			continue
		}
		// primary key id never updated by map
		if strings.EqualFold(f.DBName, "id") {
			continue
		}
		if createMode {
			if s, ok := v.(string); ok && strings.TrimSpace(s) == "" {
				// keep behavior similar to old service: empty string treated as "not provided"
				continue
			}
		}

		// status is handled by model.UpdateChannelStatus to keep ability/cache behavior consistent
		if strings.EqualFold(f.DBName, "status") {
			n := int(mgrToInt64(v))
			status = &n
			continue
		}

		converted, err := mgrConvertValue(f.FieldType, v)
		if err != nil {
			return nil, nil, err
		}

		// for pointer columns, mgrConvertValue returns underlying value; we should allow NULL with nil
		if f.FieldType.Kind() == reflect.Pointer {
			if v == nil {
				updates[f.DBName] = nil
			} else {
				updates[f.DBName] = converted
			}
		} else {
			updates[f.DBName] = converted
		}
	}
	return updates, status, nil
}

func mgrRecalcMultiKeySizeIfNeeded(ch *model.Channel, newKey string) {
	if !ch.ChannelInfo.IsMultiKey {
		return
	}
	keyStr := newKey
	if keyStr == "" {
		keyStr = ch.Key
	}
	// Use existing parsing logic (newline or JSON array).
	ch.Key = keyStr
	keys := ch.GetKeys()
	ch.ChannelInfo.MultiKeySize = len(keys)
	if ch.ChannelInfo.MultiKeyStatusList != nil {
		for idx := range ch.ChannelInfo.MultiKeyStatusList {
			if idx >= ch.ChannelInfo.MultiKeySize {
				delete(ch.ChannelInfo.MultiKeyStatusList, idx)
			}
		}
	}
}

// GET /api/channel/mgr/meta
func ChannelMgrMeta(c *gin.Context) {
	cts, err := model.DB.Migrator().ColumnTypes(&model.Channel{})
	if err != nil {
		mgrWriteError(c, http.StatusInternalServerError, err)
		return
	}

	cols := make([]mgrColumnInfo, 0, len(cts))
	for _, ct := range cts {
		name := ct.Name()
		typ := ct.DatabaseTypeName()
		notNull := false
		if nullable, ok := ct.Nullable(); ok {
			notNull = !nullable
		}
		var def *string
		if dv, ok := ct.DefaultValue(); ok {
			v := dv
			def = &v
		}
		pk := false
		if v, ok := ct.PrimaryKey(); ok {
			pk = v
		}
		cols = append(cols, mgrColumnInfo{
			Name:         name,
			Type:         typ,
			NotNull:      notNull,
			DefaultValue: def,
			PK:           pk,
		})
	}

	mgrWriteJSON(c, http.StatusOK, gin.H{"columns": cols})
}

// GET /api/channel/mgr/list?offset=&limit=&key=&tag=&status=&type=
func ChannelMgrList(c *gin.Context) {
	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	keyFilter := strings.TrimSpace(c.Query("key"))
	tagFilter := strings.TrimSpace(c.Query("tag"))
	statusFilter := strings.TrimSpace(c.Query("status"))
	typeFilter := strings.TrimSpace(c.Query("type"))

	db := model.DB.Model(&model.Channel{})
	if keyFilter != "" {
		db = db.Where(clause.Like{Column: clause.Column{Name: "key"}, Value: "%" + keyFilter + "%"})
	}
	if tagFilter != "" {
		db = db.Where(clause.Like{Column: clause.Column{Name: "tag"}, Value: "%" + tagFilter + "%"})
	}
	if statusFilter != "" {
		status, err := strconv.Atoi(statusFilter)
		if err != nil {
			mgrWriteErrorMsg(c, http.StatusBadRequest, "invalid status")
			return
		}
		if status < 0 || status > 3 {
			mgrWriteErrorMsg(c, http.StatusBadRequest, "invalid status")
			return
		}
		db = db.Where(clause.Eq{Column: clause.Column{Name: "status"}, Value: status})
	}
	if typeFilter != "" {
		tp, err := strconv.Atoi(typeFilter)
		if err != nil {
			mgrWriteErrorMsg(c, http.StatusBadRequest, "invalid type")
			return
		}
		if tp < 0 || tp >= constant.ChannelTypeDummy {
			mgrWriteErrorMsg(c, http.StatusBadRequest, "invalid type")
			return
		}
		db = db.Where(clause.Eq{Column: clause.Column{Name: "type"}, Value: tp})
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		mgrWriteError(c, http.StatusInternalServerError, err)
		return
	}

	var rows []map[string]interface{}
	if err := db.Order("id").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		mgrWriteError(c, http.StatusInternalServerError, err)
		return
	}
	mgrWriteJSON(c, http.StatusOK, gin.H{
		"rows":  rows,
		"total": total,
	})
}

// POST /api/channel/mgr/create
// body: {"values": {...}}
func ChannelMgrCreate(c *gin.Context) {
	if c.Request.Method != http.MethodPost {
		c.Status(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Values map[string]interface{} `json:"values"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		mgrWriteError(c, http.StatusBadRequest, err)
		return
	}
	if req.Values == nil {
		mgrWriteErrorMsg(c, http.StatusBadRequest, "values is required")
		return
	}

	sc, err := getChannelMgrSchema()
	if err != nil {
		mgrWriteError(c, http.StatusInternalServerError, err)
		return
	}

	ch := &model.Channel{}
	dst := reflect.ValueOf(ch).Elem()
	anySet := false
	for k, v := range req.Values {
		f := sc.byInputName[k]
		if f == nil {
			continue
		}
		if strings.EqualFold(f.DBName, "id") {
			continue
		}
		if s, ok := v.(string); ok && strings.TrimSpace(s) == "" && !strings.EqualFold(f.DBName, "id") {
			// keep old behavior: empty string on create treated as "not provided"
			continue
		}
		if err := mgrSetStructField(dst, f, v); err != nil {
			mgrWriteError(c, http.StatusBadRequest, err)
			return
		}
		anySet = true
	}
	if !anySet {
		mgrWriteErrorMsg(c, http.StatusBadRequest, "no valid columns to insert")
		return
	}
	if strings.TrimSpace(ch.Key) == "" {
		mgrWriteErrorMsg(c, http.StatusBadRequest, "key is required")
		return
	}
	if ch.CreatedTime == 0 {
		ch.CreatedTime = common.GetTimestamp()
	}
	if strings.TrimSpace(ch.Group) == "" {
		ch.Group = "default"
	}

	tx := model.DB.Begin()
	if tx.Error != nil {
		mgrWriteError(c, http.StatusInternalServerError, tx.Error)
		return
	}
	defer func() { _ = tx.Rollback() }()

	if err := tx.Create(ch).Error; err != nil {
		mgrWriteError(c, http.StatusInternalServerError, err)
		return
	}
	if err := ch.AddAbilities(tx); err != nil {
		mgrWriteError(c, http.StatusInternalServerError, err)
		return
	}
	if err := tx.Commit().Error; err != nil {
		mgrWriteError(c, http.StatusInternalServerError, err)
		return
	}
	mgrWriteJSON(c, http.StatusOK, gin.H{"id": ch.Id})
}

// POST /api/channel/mgr/update
// body: {"id": 1, "values": {...}}
func ChannelMgrUpdate(c *gin.Context) {
	if c.Request.Method != http.MethodPost {
		c.Status(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID     interface{}            `json:"id"`
		Values map[string]interface{} `json:"values"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		mgrWriteError(c, http.StatusBadRequest, err)
		return
	}
	id := int(mgrToInt64(req.ID))
	if id <= 0 {
		mgrWriteErrorMsg(c, http.StatusBadRequest, "invalid id")
		return
	}
	if req.Values == nil {
		mgrWriteErrorMsg(c, http.StatusBadRequest, "values is required")
		return
	}

	ch, err := model.GetChannelById(id, true)
	if err != nil {
		mgrWriteError(c, http.StatusNotFound, err)
		return
	}

	updates, status, err := mgrBuildUpdateMap(req.Values, false)
	if err != nil {
		mgrWriteError(c, http.StatusBadRequest, err)
		return
	}
	if len(updates) == 0 && status == nil {
		mgrWriteErrorMsg(c, http.StatusBadRequest, "no columns to update")
		return
	}

	// If key is being changed on a multi-key channel, also adjust channel_info size fields.
	if rawNewKey, ok := req.Values["key"]; ok {
		newKey, _ := rawNewKey.(string)
		mgrRecalcMultiKeySizeIfNeeded(ch, newKey)
		updates["channel_info"] = ch.ChannelInfo
	}
	if rawNewKey, ok := req.Values["open_ai_organization"]; ok && req.Values["openai_organization"] == nil {
		_ = rawNewKey
	}

	if len(updates) > 0 {
		tx := model.DB.Begin()
		if tx.Error != nil {
			mgrWriteError(c, http.StatusInternalServerError, tx.Error)
			return
		}
		defer func() { _ = tx.Rollback() }()

		if err := tx.Model(&model.Channel{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			mgrWriteError(c, http.StatusInternalServerError, err)
			return
		}

		// Keep abilities consistent if relevant fields changed (safe to call unconditionally for mgr usage).
		var updated model.Channel
		if err := tx.First(&updated, "id = ?", id).Error; err != nil {
			mgrWriteError(c, http.StatusInternalServerError, err)
			return
		}
		if err := updated.UpdateAbilities(tx); err != nil {
			mgrWriteError(c, http.StatusInternalServerError, err)
			return
		}

		if err := tx.Commit().Error; err != nil {
			mgrWriteError(c, http.StatusInternalServerError, err)
			return
		}
	}

	// status update must go through UpdateChannelStatus (cache + ability status)
	if status != nil {
		_ = model.UpdateChannelStatus(id, "", *status, "updated by channel mgr")
	}

	mgrWriteJSON(c, http.StatusOK, gin.H{"ok": true})
}

// POST /api/channel/mgr/delete
// body: {"id": 1}
// behavior: if key is non-empty, delete all rows with same key; otherwise only delete the id row
func ChannelMgrDelete(c *gin.Context) {
	if c.Request.Method != http.MethodPost {
		c.Status(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID interface{} `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		mgrWriteError(c, http.StatusBadRequest, err)
		return
	}
	id := int(mgrToInt64(req.ID))
	if id <= 0 {
		mgrWriteErrorMsg(c, http.StatusBadRequest, "invalid id")
		return
	}

	ch, err := model.GetChannelById(id, true)
	if err != nil {
		mgrWriteError(c, http.StatusNotFound, err)
		return
	}

	var ids []int
	if strings.TrimSpace(ch.Key) != "" {
		var rows []struct {
			Id int `gorm:"column:id"`
		}
		if err := model.DB.Model(&model.Channel{}).
			Select("id").
			Where(clause.Eq{Column: clause.Column{Name: "key"}, Value: ch.Key}).
			Find(&rows).Error; err != nil {
			mgrWriteError(c, http.StatusInternalServerError, err)
			return
		}
		for _, r := range rows {
			ids = append(ids, r.Id)
		}
	} else {
		ids = []int{id}
	}

	if len(ids) == 0 {
		mgrWriteErrorMsg(c, http.StatusNotFound, "row not found")
		return
	}

	if err := model.BatchDeleteChannels(ids); err != nil {
		mgrWriteError(c, http.StatusInternalServerError, err)
		return
	}

	mgrWriteJSON(c, http.StatusOK, gin.H{
		"ok":      true,
		"deleted": len(ids),
	})
}

// POST /api/channel/mgr/copy
// body: {"id": 1}
func ChannelMgrCopy(c *gin.Context) {
	if c.Request.Method != http.MethodPost {
		c.Status(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID interface{} `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		mgrWriteError(c, http.StatusBadRequest, err)
		return
	}
	id := int(mgrToInt64(req.ID))
	if id <= 0 {
		mgrWriteErrorMsg(c, http.StatusBadRequest, "invalid id")
		return
	}

	ch, err := model.GetChannelById(id, true)
	if err != nil {
		mgrWriteError(c, http.StatusNotFound, err)
		return
	}

	newCh := *ch
	newCh.Id = 0

	tx := model.DB.Begin()
	if tx.Error != nil {
		mgrWriteError(c, http.StatusInternalServerError, tx.Error)
		return
	}
	defer func() { _ = tx.Rollback() }()

	if err := tx.Create(&newCh).Error; err != nil {
		mgrWriteError(c, http.StatusInternalServerError, err)
		return
	}
	if err := newCh.AddAbilities(tx); err != nil {
		mgrWriteError(c, http.StatusInternalServerError, err)
		return
	}
	if err := tx.Commit().Error; err != nil {
		mgrWriteError(c, http.StatusInternalServerError, err)
		return
	}

	mgrWriteJSON(c, http.StatusOK, gin.H{"id": newCh.Id})
}

// POST /api/channel/mgr/batch_copy
// body: {"id": 1, "keys": ["k1", "k2"]}
// behavior: copy template row, override key, reset used_quota=0
func ChannelMgrBatchCopy(c *gin.Context) {
	if c.Request.Method != http.MethodPost {
		c.Status(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID   interface{} `json:"id"`
		Keys []string    `json:"keys"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		mgrWriteError(c, http.StatusBadRequest, err)
		return
	}
	id := int(mgrToInt64(req.ID))
	if id <= 0 {
		mgrWriteErrorMsg(c, http.StatusBadRequest, "invalid id")
		return
	}
	if len(req.Keys) == 0 {
		mgrWriteErrorMsg(c, http.StatusBadRequest, "keys is required")
		return
	}

	// clean keys: trim + dedupe
	seen := make(map[string]struct{}, len(req.Keys))
	cleaned := make([]string, 0, len(req.Keys))
	for _, k := range req.Keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		cleaned = append(cleaned, k)
	}
	if len(cleaned) == 0 {
		mgrWriteErrorMsg(c, http.StatusBadRequest, "no valid keys")
		return
	}

	ch, err := model.GetChannelById(id, true)
	if err != nil {
		mgrWriteError(c, http.StatusNotFound, err)
		return
	}

	newChannels := make([]model.Channel, 0, len(cleaned))
	for _, k := range cleaned {
		nc := *ch
		nc.Id = 0
		nc.Key = k
		nc.UsedQuota = 0
		newChannels = append(newChannels, nc)
	}

	if err := model.BatchInsertChannels(newChannels); err != nil {
		mgrWriteError(c, http.StatusInternalServerError, err)
		return
	}
	mgrWriteJSON(c, http.StatusOK, gin.H{"ok": true, "count": len(newChannels)})
}

// POST /api/channel/mgr/batch_update
// body: {"ids":[1,2], "values": {...}}
func ChannelMgrBatchUpdate(c *gin.Context) {
	if c.Request.Method != http.MethodPost {
		c.Status(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		IDs    []interface{}          `json:"ids"`
		Values map[string]interface{} `json:"values"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		mgrWriteError(c, http.StatusBadRequest, err)
		return
	}
	if len(req.IDs) == 0 {
		mgrWriteErrorMsg(c, http.StatusBadRequest, "ids is required")
		return
	}
	if len(req.Values) == 0 {
		mgrWriteErrorMsg(c, http.StatusBadRequest, "values is required")
		return
	}

	ids := make([]int, 0, len(req.IDs))
	seen := make(map[int]struct{}, len(req.IDs))
	for _, v := range req.IDs {
		id := int(mgrToInt64(v))
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		mgrWriteErrorMsg(c, http.StatusBadRequest, "no valid ids")
		return
	}

	updates, status, err := mgrBuildUpdateMap(req.Values, false)
	if err != nil {
		mgrWriteError(c, http.StatusBadRequest, err)
		return
	}
	if len(updates) == 0 && status == nil {
		mgrWriteErrorMsg(c, http.StatusBadRequest, "no columns to update")
		return
	}

	if len(updates) > 0 {
		if err := model.DB.Model(&model.Channel{}).Where("id IN ?", ids).Updates(updates).Error; err != nil {
			mgrWriteError(c, http.StatusInternalServerError, err)
			return
		}
		// Abilities may depend on several fields; for mgr usage we keep it safe and update per channel.
		for _, id := range ids {
			ch, err := model.GetChannelById(id, true)
			if err != nil {
				continue
			}
			_ = ch.UpdateAbilities(nil)
		}
	}

	if status != nil {
		for _, id := range ids {
			_ = model.UpdateChannelStatus(id, "", *status, "batch updated by channel mgr")
		}
	}

	mgrWriteJSON(c, http.StatusOK, gin.H{"ok": true, "count": len(ids)})
}

var _ = errors.New
