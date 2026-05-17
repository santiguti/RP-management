package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/santiguti/rp-management/backend/internal/auth"
	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
)

func TestBrands_ListSeedData(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	res, err := client.Get(ts.URL + "/api/v1/brands")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}
	var body struct {
		Brands []lookupDTO `json:"brands"`
	}
	decodeJSON(t, res.Body, &body)
	if len(body.Brands) < 16 {
		t.Fatalf("len(brands) = %d, want at least 16 seed brands", len(body.Brands))
	}
	names := lookupNames(body.Brands)
	if !hasName(names, "Samsung") || !hasName(names, "Apple") {
		t.Fatalf("brand names = %v, want Samsung and Apple", names)
	}
}

func TestBrands_CreateAsOwner(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	name := uniqueLookupName("Marca")
	res := postJSON(t, client, ts.URL+"/api/v1/brands", map[string]string{"name": "  " + name + "  "}, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusCreated, readBody(t, res))
	}
	var body struct {
		Brand lookupDTO `json:"brand"`
	}
	decodeJSON(t, res.Body, &body)
	if body.Brand.Name != name || body.Brand.Ucode == "" {
		t.Fatalf("brand = %+v, want trimmed name and ucode", body.Brand)
	}
}

func TestBrands_CreateAsEmployee_403(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	employee := seedUserWithRole(t, q, "employee")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, employee.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/brands", map[string]string{"name": uniqueLookupName("Marca")}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusForbidden, "forbidden")
}

func TestBrands_DuplicateName_409(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/brands", map[string]string{"name": "Samsung"}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusConflict, "already_exists")
}

func TestDeviceModels_RequireValidBrand(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/device-models", map[string]string{
		"brand_ucode": "00000000-0000-0000-0000-000000000000",
		"name":        "Modelo X",
	}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusNotFound, "not_found")
}

func TestDeviceModels_CreateAndListByBrand(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	brand := seedBrand(t, q, uniqueLookupName("MarcaModelo"))
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	modelName := uniqueLookupName("Modelo")
	res := postJSON(t, client, ts.URL+"/api/v1/device-models", map[string]string{
		"brand_ucode": uuidString(brand.Ucode),
		"name":        modelName,
	}, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusCreated, readBody(t, res))
	}
	var createBody struct {
		DeviceModel deviceModelDTO `json:"device_model"`
	}
	decodeJSON(t, res.Body, &createBody)
	if createBody.DeviceModel.BrandUcode != uuidString(brand.Ucode) {
		t.Fatalf("brand_ucode = %q, want %q", createBody.DeviceModel.BrandUcode, uuidString(brand.Ucode))
	}

	list, err := client.Get(ts.URL + "/api/v1/brands/" + uuidString(brand.Ucode) + "/models")
	if err != nil {
		t.Fatal(err)
	}
	defer list.Body.Close()
	if list.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d, want %d: %s", list.StatusCode, http.StatusOK, readBody(t, list))
	}
	var listBody struct {
		DeviceModels []deviceModelDTO `json:"device_models"`
	}
	decodeJSON(t, list.Body, &listBody)
	if len(listBody.DeviceModels) != 1 || listBody.DeviceModels[0].Name != modelName {
		t.Fatalf("device_models = %+v, want one %q", listBody.DeviceModels, modelName)
	}
}

func TestDeviceModels_DuplicateName_409(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	brand := seedBrand(t, q, uniqueLookupName("MarcaModelo"))
	modelName := uniqueLookupName("Modelo")
	seedDeviceModel(t, q, brand.ID, modelName)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/device-models", map[string]string{
		"brand_ucode": uuidString(brand.Ucode),
		"name":        modelName,
	}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusConflict, "already_exists")
}

func TestArticleTypes_CreateAsOwner(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	name := uniqueLookupName("tipo")
	res := postJSON(t, client, ts.URL+"/api/v1/article-types", map[string]string{"name": name}, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusCreated, readBody(t, res))
	}
	var body struct {
		ArticleType lookupDTO `json:"article_type"`
	}
	decodeJSON(t, res.Body, &body)
	if body.ArticleType.Name != name || body.ArticleType.Ucode == "" {
		t.Fatalf("article_type = %+v, want created DTO", body.ArticleType)
	}
}

func TestArticleTypes_CreateAsEmployee_403(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	employee := seedUserWithRole(t, q, "employee")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, employee.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/article-types", map[string]string{"name": uniqueLookupName("tipo")}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusForbidden, "forbidden")
}

func TestArticleTypes_DuplicateName_409(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/article-types", map[string]string{"name": "celular"}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusConflict, "already_exists")
}

func TestArticleTypes_UpdateAndDelete(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	articleType := seedArticleType(t, q, uniqueLookupName("tipo"))
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	updatedName := uniqueLookupName("tipo_actualizado")
	patch := patchJSON(t, client, ts.URL+"/api/v1/article-types/"+uuidString(articleType.Ucode), map[string]string{"name": updatedName}, csrf)
	defer patch.Body.Close()
	if patch.StatusCode != http.StatusOK {
		t.Fatalf("patch status = %d, want %d: %s", patch.StatusCode, http.StatusOK, readBody(t, patch))
	}
	var patchBody struct {
		ArticleType lookupDTO `json:"article_type"`
	}
	decodeJSON(t, patch.Body, &patchBody)
	if patchBody.ArticleType.Name != updatedName {
		t.Fatalf("article_type.name = %q, want %q", patchBody.ArticleType.Name, updatedName)
	}

	del := deleteReq(t, client, ts.URL+"/api/v1/article-types/"+uuidString(articleType.Ucode), csrf)
	defer del.Body.Close()
	if del.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d: %s", del.StatusCode, http.StatusNoContent, readBody(t, del))
	}
}

func seedUserWithRole(t *testing.T, q *sqlc.Queries, role string) sqlc.User {
	t.Helper()

	hash, err := auth.Hash("pw")
	if err != nil {
		t.Fatal(err)
	}
	user, err := q.CreateUser(context.Background(), sqlc.CreateUserParams{
		Username:        uniqueUsername(t),
		PasswordHash:    hash,
		FullName:        "Test " + strings.Title(role),
		Role:            role,
		CreatedByUserID: pgtype.Int8{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return user
}

func seedBrand(t *testing.T, q *sqlc.Queries, name string) sqlc.Brand {
	t.Helper()

	brand, err := q.CreateBrand(context.Background(), sqlc.CreateBrandParams{
		Name:            name,
		CreatedByUserID: pgtype.Int8{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return brand
}

func seedDeviceModel(t *testing.T, q *sqlc.Queries, brandID int64, name string) sqlc.DeviceModel {
	t.Helper()

	model, err := q.CreateDeviceModel(context.Background(), sqlc.CreateDeviceModelParams{
		BrandID:         brandID,
		Name:            name,
		CreatedByUserID: pgtype.Int8{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return model
}

func seedArticleType(t *testing.T, q *sqlc.Queries, name string) sqlc.ArticleType {
	t.Helper()

	articleType, err := q.CreateArticleType(context.Background(), sqlc.CreateArticleTypeParams{
		Name:            name,
		CreatedByUserID: pgtype.Int8{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return articleType
}

func lookupNames(items []lookupDTO) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}

func hasName(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}

func uniqueLookupName(prefix string) string {
	return fmt.Sprintf("%s %d", prefix, time.Now().UnixNano())
}
