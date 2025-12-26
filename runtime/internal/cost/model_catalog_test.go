package cost

import (
	"errors"
	"testing"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

func TestModelCatalog_Get(t *testing.T) {
	catalog := NewModelCatalog()

	tests := []struct {
		name    string
		modelID contracts.ModelID
		wantOK  bool
	}{
		{"existing opus 4.5", "claude-opus-4-5-20251101", true},
		{"existing sonnet 4.5", "claude-sonnet-4-5-20250929", true},
		{"existing haiku 3", "claude-3-haiku-20240307", true},
		{"non-existing model", "unknown-model", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, ok := catalog.Get(tt.modelID)
			if ok != tt.wantOK {
				t.Errorf("Get(%s) ok = %v, want %v", tt.modelID, ok, tt.wantOK)
			}
			if ok && info.ID != tt.modelID {
				t.Errorf("Get(%s) ID = %v, want %v", tt.modelID, info.ID, tt.modelID)
			}
		})
	}
}

func TestModelCatalog_GetByRole(t *testing.T) {
	catalog := NewModelCatalog()

	tests := []struct {
		name       string
		role       contracts.ModelRole
		wantModel  contracts.ModelID
		wantOK     bool
	}{
		{"flagship", contracts.RoleFlagship, "claude-opus-4-5-20251101", true},
		{"balanced", contracts.RoleBalanced, "claude-sonnet-4-5-20250929", true},
		{"fast", contracts.RoleFast, "claude-3-haiku-20240307", true},
		{"unknown", contracts.ModelRole("unknown"), "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, ok := catalog.GetByRole(tt.role)
			if ok != tt.wantOK {
				t.Errorf("GetByRole(%s) ok = %v, want %v", tt.role, ok, tt.wantOK)
			}
			if ok && info.ID != tt.wantModel {
				t.Errorf("GetByRole(%s) ID = %v, want %v", tt.role, info.ID, tt.wantModel)
			}
		})
	}
}

func TestModelCatalog_List(t *testing.T) {
	catalog := NewModelCatalog()

	models := catalog.List()
	if len(models) != len(DefaultModels) {
		t.Errorf("List() returned %d models, want %d", len(models), len(DefaultModels))
	}

	// Verify all default models are present
	modelMap := make(map[contracts.ModelID]bool)
	for _, m := range models {
		modelMap[m.ID] = true
	}

	for _, m := range DefaultModels {
		if !modelMap[m.ID] {
			t.Errorf("List() missing model %s", m.ID)
		}
	}
}

func TestModelCatalog_SetRoleMapping(t *testing.T) {
	catalog := NewModelCatalog()

	// Change flagship to haiku
	err := catalog.SetRoleMapping(contracts.RoleFlagship, "claude-3-haiku-20240307")
	if err != nil {
		t.Fatalf("SetRoleMapping() error = %v", err)
	}

	info, ok := catalog.GetByRole(contracts.RoleFlagship)
	if !ok {
		t.Fatal("GetByRole() after SetRoleMapping failed")
	}
	if info.ID != "claude-3-haiku-20240307" {
		t.Errorf("GetByRole() after SetRoleMapping = %v, want claude-3-haiku-20240307", info.ID)
	}
}

func TestModelCatalog_SetRoleMappingUnknownModel(t *testing.T) {
	catalog := NewModelCatalog()

	err := catalog.SetRoleMapping(contracts.RoleFlagship, "unknown-model")
	if !errors.Is(err, contracts.ErrModelUnknown) {
		t.Errorf("SetRoleMapping() error = %v, want ErrModelUnknown", err)
	}
}

func TestModelCatalog_CustomModels(t *testing.T) {
	customModels := []contracts.ModelInfo{
		{
			ID:              "my-model",
			Provider:        "custom",
			MaxContext:      10000,
			InputCostPer1M:  1.0,
			OutputCostPer1M: 2.0,
			DefaultRole:     contracts.RoleFlagship,
			SupportsTools:   false,
		},
	}
	customMappings := map[contracts.ModelRole]contracts.ModelID{
		contracts.RoleFlagship: "my-model",
	}

	catalog := NewModelCatalogWithModels(customModels, customMappings)

	// Custom model should exist
	info, ok := catalog.Get("my-model")
	if !ok {
		t.Fatal("Get() failed for custom model")
	}
	if info.Provider != "custom" {
		t.Errorf("Provider = %v, want custom", info.Provider)
	}

	// Default model should not exist
	_, ok = catalog.Get("claude-opus-4-5-20251101")
	if ok {
		t.Error("Get() should return false for default model in custom catalog")
	}
}

func TestModelInfo_AverageCostPer1M(t *testing.T) {
	info := contracts.ModelInfo{
		InputCostPer1M:  10.0,
		OutputCostPer1M: 30.0,
	}

	avg := info.AverageCostPer1M()
	if avg != 20.0 {
		t.Errorf("AverageCostPer1M() = %v, want 20.0", avg)
	}
}

func TestModelCatalog_DefaultRoleMappings(t *testing.T) {
	// Verify default mappings match expected roles
	catalog := NewModelCatalog()

	flagship, _ := catalog.GetByRole(contracts.RoleFlagship)
	if flagship.DefaultRole != contracts.RoleFlagship {
		t.Errorf("flagship model's DefaultRole = %v, want flagship", flagship.DefaultRole)
	}

	balanced, _ := catalog.GetByRole(contracts.RoleBalanced)
	if balanced.DefaultRole != contracts.RoleBalanced {
		t.Errorf("balanced model's DefaultRole = %v, want balanced", balanced.DefaultRole)
	}

	fast, _ := catalog.GetByRole(contracts.RoleFast)
	if fast.DefaultRole != contracts.RoleFast {
		t.Errorf("fast model's DefaultRole = %v, want fast", fast.DefaultRole)
	}
}
