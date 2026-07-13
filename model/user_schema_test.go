package model

import (
	"reflect"
	"testing"
)

func TestUserProfileColumnLengthsMatchSQLSnapshot(t *testing.T) {
	userType := reflect.TypeOf(User{})

	tests := []struct {
		fieldName string
		gormType  string
		max       string
	}{
		{fieldName: "Avatar", gormType: "type:varchar(2048);column:avatar", max: "max=2048"},
		{fieldName: "PhoneCountryCode", gormType: "type:varchar(16);column:phone_country_code", max: "max=16"},
		{fieldName: "PhoneNumber", gormType: "type:varchar(32);column:phone_number", max: "max=32"},
	}

	for _, tt := range tests {
		t.Run(tt.fieldName, func(t *testing.T) {
			field, ok := userType.FieldByName(tt.fieldName)
			if !ok {
				t.Fatalf("field %s not found", tt.fieldName)
			}

			if got := field.Tag.Get("gorm"); got != tt.gormType {
				t.Fatalf("gorm tag = %q, want %q", got, tt.gormType)
			}
			if got := field.Tag.Get("validate"); got != tt.max {
				t.Fatalf("validate tag = %q, want %q", got, tt.max)
			}
		})
	}
}

func TestFillUserProviderNames(t *testing.T) {
	tx := DB.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()

	provider := Provider{OwnerUserId: 987654321, Name: "Provider Site"}
	if err := tx.Create(&provider).Error; err != nil {
		t.Fatal(err)
	}
	users := []*User{
		{ProviderId: 0},
		{ProviderId: provider.Id},
		{ProviderId: provider.Id},
		{ProviderId: provider.Id + 1000000},
	}

	if err := fillUserProviderNames(tx, users); err != nil {
		t.Fatal(err)
	}
	if users[0].ProviderName != "" {
		t.Fatalf("main-site provider name = %q, want empty", users[0].ProviderName)
	}
	if users[1].ProviderName != provider.Name || users[2].ProviderName != provider.Name {
		t.Fatalf("provider names = %q, %q, want %q", users[1].ProviderName, users[2].ProviderName, provider.Name)
	}
	if users[3].ProviderName != "" {
		t.Fatalf("missing provider name = %q, want empty", users[3].ProviderName)
	}
}
