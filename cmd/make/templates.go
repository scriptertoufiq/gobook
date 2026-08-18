package main

// Templates below use `~` wherever the generated code needs a backtick, since
// Go raw strings can't contain one. render() swaps them back.

const modelTemplate = `package models

// {{.Pascal}} — TODO: replace these placeholder fields with the real schema,
// then run: go run ./cmd/migrate
type {{.Pascal}} struct {
	Base
	Name        string ~gorm:"size:120;not null;index" json:"name"~
	Slug        string ~gorm:"size:140;not null;uniqueIndex" json:"slug"~
	Description string ~gorm:"type:text" json:"description"~
}

func ({{.Pascal}}) TableName() string { return "{{.PluralSnake}}" }
`

const repositoryTemplate = `package repositories

import (
	"context"

	"gorm.io/gorm"

	"{{.Module}}/internal/models"
	"{{.Module}}/pkg/pagination"
)

type {{.Pascal}}Repository interface {
	Create(ctx context.Context, {{.Camel}} *models.{{.Pascal}}) error
	Update(ctx context.Context, {{.Camel}} *models.{{.Pascal}}) error
	Delete(ctx context.Context, id uint) error
	FindByID(ctx context.Context, id uint) (*models.{{.Pascal}}, error)
	SlugTaken(ctx context.Context, slug string, excludeID uint) (bool, error)
	Paginate(ctx context.Context, p pagination.Params) ([]models.{{.Pascal}}, int64, error)
}

type {{.Camel}}Repository struct {
	db *gorm.DB
}

func New{{.Pascal}}Repository(db *gorm.DB) {{.Pascal}}Repository {
	return &{{.Camel}}Repository{db: db}
}

// {{.Camel}}Sortable whitelists the columns clients may sort by.
var {{.Camel}}Sortable = []string{"id", "name", "created_at"}

func (r *{{.Camel}}Repository) Create(ctx context.Context, {{.Camel}} *models.{{.Pascal}}) error {
	return r.db.WithContext(ctx).Create({{.Camel}}).Error
}

func (r *{{.Camel}}Repository) Update(ctx context.Context, {{.Camel}} *models.{{.Pascal}}) error {
	return r.db.WithContext(ctx).Save({{.Camel}}).Error
}

func (r *{{.Camel}}Repository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.{{.Pascal}}{}, id).Error
}

func (r *{{.Camel}}Repository) FindByID(ctx context.Context, id uint) (*models.{{.Pascal}}, error) {
	var {{.Camel}} models.{{.Pascal}}
	if err := r.db.WithContext(ctx).First(&{{.Camel}}, id).Error; err != nil {
		return nil, err
	}
	return &{{.Camel}}, nil
}

func (r *{{.Camel}}Repository) SlugTaken(ctx context.Context, slug string, excludeID uint) (bool, error) {
	query := r.db.WithContext(ctx).Model(&models.{{.Pascal}}{}).Where("slug = ?", slug)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *{{.Camel}}Repository) Paginate(ctx context.Context, p pagination.Params) ([]models.{{.Pascal}}, int64, error) {
	query := r.db.WithContext(ctx).Model(&models.{{.Pascal}}{})

	if p.Search != "" {
		query = query.Where("name LIKE ?", "%"+p.Search+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var {{.PluralCamel}} []models.{{.Pascal}}
	err := query.
		Order(p.OrderClause({{.Camel}}Sortable, "id")).
		Limit(p.PerPage).
		Offset(p.Offset()).
		Find(&{{.PluralCamel}}).Error

	return {{.PluralCamel}}, total, err
}
`

const requestTemplate = `package requests

type Create{{.Pascal}}Request struct {
	Name        string ~json:"name" binding:"required,min=2,max=120"~
	Slug        string ~json:"slug" binding:"omitempty,max=140"~
	Description string ~json:"description" binding:"omitempty"~
}

// Pointers make "field absent" distinguishable from "field set to its zero
// value", so a PATCH only touches what the client actually sent.
type Update{{.Pascal}}Request struct {
	Name        *string ~json:"name" binding:"omitempty,min=2,max=120"~
	Slug        *string ~json:"slug" binding:"omitempty,max=140"~
	Description *string ~json:"description" binding:"omitempty"~
}
`

const resourceTemplate = `package resources

import (
	"time"

	"{{.Module}}/internal/models"
)

type {{.Pascal}}Resource struct {
	ID          uint      ~json:"id"~
	Name        string    ~json:"name"~
	Slug        string    ~json:"slug"~
	Description string    ~json:"description"~
	CreatedAt   time.Time ~json:"created_at"~
	UpdatedAt   time.Time ~json:"updated_at"~
}

func New{{.Pascal}}Resource({{.Camel}} *models.{{.Pascal}}) {{.Pascal}}Resource {
	return {{.Pascal}}Resource{
		ID:          {{.Camel}}.ID,
		Name:        {{.Camel}}.Name,
		Slug:        {{.Camel}}.Slug,
		Description: {{.Camel}}.Description,
		CreatedAt:   {{.Camel}}.CreatedAt,
		UpdatedAt:   {{.Camel}}.UpdatedAt,
	}
}

func New{{.Pascal}}Collection({{.PluralCamel}} []models.{{.Pascal}}) []{{.Pascal}}Resource {
	out := make([]{{.Pascal}}Resource, 0, len({{.PluralCamel}}))
	for i := range {{.PluralCamel}} {
		out = append(out, New{{.Pascal}}Resource(&{{.PluralCamel}}[i]))
	}
	return out
}
`

const serviceTemplate = `package services

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"{{.Module}}/internal/models"
	"{{.Module}}/internal/repositories"
	"{{.Module}}/internal/requests"
	"{{.Module}}/pkg/apperror"
	"{{.Module}}/pkg/pagination"
)

type {{.Pascal}}Service struct {
	repo repositories.{{.Pascal}}Repository
}

func New{{.Pascal}}Service(repo repositories.{{.Pascal}}Repository) *{{.Pascal}}Service {
	return &{{.Pascal}}Service{repo: repo}
}

func (s *{{.Pascal}}Service) List(ctx context.Context, p pagination.Params) ([]models.{{.Pascal}}, pagination.Meta, error) {
	{{.PluralCamel}}, total, err := s.repo.Paginate(ctx, p)
	if err != nil {
		return nil, pagination.Meta{}, apperror.Internal(err)
	}
	return {{.PluralCamel}}, pagination.NewMeta(p, total), nil
}

func (s *{{.Pascal}}Service) Get(ctx context.Context, id uint) (*models.{{.Pascal}}, error) {
	{{.Camel}}, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("{{.Pascal}}")
		}
		return nil, apperror.Internal(err)
	}
	return {{.Camel}}, nil
}

func (s *{{.Pascal}}Service) Create(ctx context.Context, req requests.Create{{.Pascal}}Request) (*models.{{.Pascal}}, error) {
	slug := req.Slug
	if slug == "" {
		slug = Slugify(req.Name)
	}

	taken, err := s.repo.SlugTaken(ctx, slug, 0)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if taken {
		return nil, apperror.Conflict("A {{.Snake}} with that slug already exists.")
	}

	{{.Camel}} := &models.{{.Pascal}}{
		Name:        strings.TrimSpace(req.Name),
		Slug:        slug,
		Description: req.Description,
	}

	if err := s.repo.Create(ctx, {{.Camel}}); err != nil {
		return nil, apperror.Internal(err)
	}
	return {{.Camel}}, nil
}

func (s *{{.Pascal}}Service) Update(ctx context.Context, id uint, req requests.Update{{.Pascal}}Request) (*models.{{.Pascal}}, error) {
	{{.Camel}}, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Slug != nil {
		slug := *req.Slug
		if slug == "" {
			slug = Slugify({{.Camel}}.Name)
		}

		taken, err := s.repo.SlugTaken(ctx, slug, {{.Camel}}.ID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if taken {
			return nil, apperror.Conflict("A {{.Snake}} with that slug already exists.")
		}
		{{.Camel}}.Slug = slug
	}

	if req.Name != nil {
		{{.Camel}}.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		{{.Camel}}.Description = *req.Description
	}

	if err := s.repo.Update(ctx, {{.Camel}}); err != nil {
		return nil, apperror.Internal(err)
	}
	return {{.Camel}}, nil
}

func (s *{{.Pascal}}Service) Delete(ctx context.Context, id uint) error {
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return apperror.Internal(err)
	}
	return nil
}
`

const controllerTemplate = `package controllers

import (
	"github.com/gin-gonic/gin"

	"{{.Module}}/internal/requests"
	"{{.Module}}/internal/resources"
	"{{.Module}}/internal/services"
	"{{.Module}}/pkg/pagination"
	"{{.Module}}/pkg/response"
)

type {{.Pascal}}Controller struct {
	service *services.{{.Pascal}}Service
}

func New{{.Pascal}}Controller(service *services.{{.Pascal}}Service) *{{.Pascal}}Controller {
	return &{{.Pascal}}Controller{service: service}
}

// Index handles GET /api/v1/{{.PluralKebab}}
func (ctrl *{{.Pascal}}Controller) Index(c *gin.Context) {
	{{.PluralCamel}}, meta, err := ctrl.service.List(c.Request.Context(), pagination.FromQuery(c.Query))
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Paginated(c, resources.New{{.Pascal}}Collection({{.PluralCamel}}), meta)
}

// Show handles GET /api/v1/{{.PluralKebab}}/:id
func (ctrl *{{.Pascal}}Controller) Show(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}

	{{.Camel}}, err := ctrl.service.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, resources.New{{.Pascal}}Resource({{.Camel}}))
}

// Store handles POST /api/v1/{{.PluralKebab}}
func (ctrl *{{.Pascal}}Controller) Store(c *gin.Context) {
	var req requests.Create{{.Pascal}}Request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	{{.Camel}}, err := ctrl.service.Create(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Created(c, resources.New{{.Pascal}}Resource({{.Camel}}))
}

// Update handles PATCH /api/v1/{{.PluralKebab}}/:id
func (ctrl *{{.Pascal}}Controller) Update(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}

	var req requests.Update{{.Pascal}}Request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	{{.Camel}}, err := ctrl.service.Update(c.Request.Context(), id, req)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, resources.New{{.Pascal}}Resource({{.Camel}}))
}

// Destroy handles DELETE /api/v1/{{.PluralKebab}}/:id
func (ctrl *{{.Pascal}}Controller) Destroy(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}

	if err := ctrl.service.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}

	response.Message(c, "{{.Pascal}} deleted.")
}
`

const serviceTestTemplate = `package services_test

import (
	"context"
	"testing"

	"gorm.io/gorm"

	"{{.Module}}/internal/models"
	"{{.Module}}/internal/repositories"
	"{{.Module}}/internal/requests"
	"{{.Module}}/internal/services"
	"{{.Module}}/pkg/pagination"
)

// fake{{.Pascal}}Repo is an in-memory stand-in for the real repository, so the
// business rules can be tested without a database.
type fake{{.Pascal}}Repo struct {
	items  map[uint]*models.{{.Pascal}}
	nextID uint
}

func newFake{{.Pascal}}Repo() *fake{{.Pascal}}Repo {
	return &fake{{.Pascal}}Repo{items: map[uint]*models.{{.Pascal}}{}, nextID: 1}
}

var _ repositories.{{.Pascal}}Repository = (*fake{{.Pascal}}Repo)(nil)

func (f *fake{{.Pascal}}Repo) Create(_ context.Context, item *models.{{.Pascal}}) error {
	item.ID = f.nextID
	f.nextID++
	f.items[item.ID] = item
	return nil
}

func (f *fake{{.Pascal}}Repo) Update(_ context.Context, item *models.{{.Pascal}}) error {
	f.items[item.ID] = item
	return nil
}

func (f *fake{{.Pascal}}Repo) Delete(_ context.Context, id uint) error {
	delete(f.items, id)
	return nil
}

func (f *fake{{.Pascal}}Repo) FindByID(_ context.Context, id uint) (*models.{{.Pascal}}, error) {
	if item, ok := f.items[id]; ok {
		return item, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fake{{.Pascal}}Repo) SlugTaken(_ context.Context, slug string, excludeID uint) (bool, error) {
	for _, item := range f.items {
		if item.Slug == slug && item.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}

func (f *fake{{.Pascal}}Repo) Paginate(_ context.Context, _ pagination.Params) ([]models.{{.Pascal}}, int64, error) {
	out := make([]models.{{.Pascal}}, 0, len(f.items))
	for _, item := range f.items {
		out = append(out, *item)
	}
	return out, int64(len(out)), nil
}

func Test{{.Pascal}}CreateDerivesSlug(t *testing.T) {
	svc := services.New{{.Pascal}}Service(newFake{{.Pascal}}Repo())

	created, err := svc.Create(context.Background(), requests.Create{{.Pascal}}Request{Name: "Hello World"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if created.Slug != "hello-world" {
		t.Errorf("expected slug %q, got %q", "hello-world", created.Slug)
	}
}

func Test{{.Pascal}}CreateRejectsDuplicateSlug(t *testing.T) {
	svc := services.New{{.Pascal}}Service(newFake{{.Pascal}}Repo())
	req := requests.Create{{.Pascal}}Request{Name: "Duplicate"}

	if _, err := svc.Create(context.Background(), req); err != nil {
		t.Fatalf("first create: %v", err)
	}

	if _, err := svc.Create(context.Background(), req); err == nil {
		t.Fatal("expected a conflict on the second create, got nil")
	}
}
`

// migrationTemplate is the blank migration `go run ./cmd/make migration` writes.
const migrationTemplate = `package migrations

import (
	"gorm.io/gorm"
	// "{{.Module}}/internal/models"
)

func init() {
	Register(Migration{
		// Never change this once it has run anywhere — it is the ledger key,
		// and renaming it makes the migration apply a second time.
		ID: "{{.MigrationID}}",

		Up: func(db *gorm.DB) error {
			// Schema follows the models, so most changes are one line:
			//
			//	return db.AutoMigrate(&models.Thing{})
			//
			// AutoMigrate only ever adds. For everything else:
			//
			//	db.Migrator().AddColumn(&models.Thing{}, "Status")
			//	db.Migrator().DropColumn(&models.Thing{}, "subtitle")
			//	db.Migrator().RenameColumn(&models.Thing{}, "body", "content")
			//	db.Exec("UPDATE things SET status = ? WHERE status = ''", "draft")
			return nil
		},

		// Reverses Up. Set this to nil if the change genuinely cannot be undone
		// — rollback then refuses the batch instead of half-completing it.
		Down: func(db *gorm.DB) error {
			return nil
		},
	})
}
`

// createTableTemplate is the migration generated alongside a new model.
const createTableTemplate = `package migrations

import (
	"gorm.io/gorm"

	"{{.Module}}/internal/models"
)

func init() {
	Register(Migration{
		ID: "{{.MigrationID}}",

		// Built from the struct, so internal/models/{{.Snake}}.go stays the
		// single description of the schema. Editing those fields later does
		// nothing on a database where this has already run — each subsequent
		// change needs its own migration:
		//
		//	go run ./cmd/make migration add_something_to_{{.PluralSnake}}
		Up: func(db *gorm.DB) error {
			return db.AutoMigrate(&models.{{.Pascal}}{})
		},

		Down: func(db *gorm.DB) error {
			return db.Migrator().DropTable(&models.{{.Pascal}}{})
		},
	})
}
`
