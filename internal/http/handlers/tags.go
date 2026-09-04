package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)
 
type TagsHandler struct {
	DB *pgxpool.Pool
}
 
type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
 
// GetTags godoc
// @Summary      Get a list of tags
// @Description  Returns a list of all available tags in the system. Each tag includes its ID and name.
// @Tags         tags
// @Produce      json
// @Success      200  {object}  map[string][]Tag  "Returns a JSON object containing a list of tags"
// @Failure      500  {string}  string  "Internal Server Error: failed to fetch tags"
// @Router       /tags [get]
func (h *TagsHandler) GetTags(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
 
	rows, err := h.DB.Query(ctx, `SELECT id, name FROM tags ORDER BY name ASC`)
	if err != nil {
		http.Error(w, "failed to fetch tags", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
 
	tags := make([]Tag, 0)
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name); err != nil {
			http.Error(w, "failed to scan tag", http.StatusInternalServerError)
			return
		}
		tags = append(tags, t)
	}
 
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"tags": tags,
	})
}