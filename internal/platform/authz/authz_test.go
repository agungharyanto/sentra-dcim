package authz

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/agungharyanto/sentra-dcim/internal/platform/principal"
	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
)

func TestScope(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	global := principal.Principal{Permissions: rbac.Resolve([]string{rbac.RoleViewer})}
	scoped := principal.Principal{Sites: map[uuid.UUID][]rbac.Permission{a: rbac.ResolveSite(rbac.RoleTechnician)}}

	if s := For(global, rbac.FacilityRead); !s.All || !s.Allows(b) {
		t.Errorf("global viewer: %+v", s)
	}
	if s := For(global, rbac.FacilityManage); !s.Empty() {
		t.Errorf("viewer manage scope must be empty: %+v", s)
	}
	s := For(scoped, rbac.RackRead)
	if s.All || !s.Allows(a) || s.Allows(b) || len(s.IDs()) != 1 {
		t.Errorf("scoped technician: %+v", s)
	}
	if !scoped.Can(rbac.RackRead) || scoped.Can(rbac.RackManage) {
		t.Error("Can must follow site grants")
	}
	// A site assignment never reaches global permissions, even if the map were filled with them.
	odd := principal.Principal{Sites: map[uuid.UUID][]rbac.Permission{a: {rbac.UsersManage}}}
	if odd.Can(rbac.UsersManage) || !For(odd, rbac.UsersManage).Empty() {
		t.Error("global permission granted through a site")
	}
	if got := scoped.Effective(); len(got) != 2 {
		t.Errorf("effective = %v", got)
	}
	if !FromContext(context.Background(), rbac.FacilityRead).Empty() {
		t.Error("no principal must give an empty scope")
	}
	ctx := principal.With(context.Background(), scoped)
	if !FromContext(ctx, rbac.FacilityRead).Allows(a) {
		t.Error("FromContext")
	}
	if (Scope{}).IDs() == nil {
		t.Error("IDs must never be nil")
	}
}
