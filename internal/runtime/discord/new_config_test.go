package discordruntime

import (
	"context"
	"errors"
	"testing"
)

func TestStartSelectedRolesSkipsGatewayWhenDisabled(t *testing.T) {
	t.Parallel()

	openGatewayCalled := false
	startSchedulerCalled := false

	err := startSelectedRoles(
		context.Background(),
		false,
		func(context.Context) error {
			openGatewayCalled = true
			return nil
		},
		true,
		func(context.Context) {
			startSchedulerCalled = true
		},
	)
	if err != nil {
		t.Fatalf("startSelectedRoles: %v", err)
	}
	if openGatewayCalled {
		t.Fatal("expected gateway open to be skipped when the gateway role is disabled")
	}
	if !startSchedulerCalled {
		t.Fatal("expected scheduler start when the scheduler role is enabled")
	}
}

func TestStartSelectedRolesReturnsGatewayError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")
	startSchedulerCalled := false

	err := startSelectedRoles(
		context.Background(),
		true,
		func(context.Context) error {
			return wantErr
		},
		true,
		func(context.Context) {
			startSchedulerCalled = true
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected gateway error %v, got %v", wantErr, err)
	}
	if startSchedulerCalled {
		t.Fatal("expected scheduler start to be skipped after gateway startup failure")
	}
}

func TestSyncGatewayCommandsSkipsRegistrationWhenGatewayDisabled(t *testing.T) {
	t.Parallel()

	registerCalled := false
	registerCachedCalled := false

	if err := syncGatewayCommands(
		context.Background(),
		false,
		false,
		nil,
		func(context.Context) error {
			registerCalled = true
			return nil
		},
		func(context.Context) error {
			registerCachedCalled = true
			return nil
		},
	); err != nil {
		t.Fatalf("syncGatewayCommands: %v", err)
	}
	if registerCalled {
		t.Fatal("expected gateway command registration to be skipped when the gateway role is disabled")
	}
	if registerCachedCalled {
		t.Fatal("expected cached-guild command registration to be skipped when the gateway role is disabled")
	}
}

func TestSyncGatewayCommandsRegistersCachedGuildsForGatewayRole(t *testing.T) {
	t.Parallel()

	registerCalled := false
	registerCachedCalled := false

	if err := syncGatewayCommands(
		context.Background(),
		true,
		true,
		nil,
		func(context.Context) error {
			registerCalled = true
			return nil
		},
		func(context.Context) error {
			registerCachedCalled = true
			return nil
		},
	); err != nil {
		t.Fatalf("syncGatewayCommands: %v", err)
	}
	if !registerCalled {
		t.Fatal("expected gateway command registration when the gateway role is enabled")
	}
	if !registerCachedCalled {
		t.Fatal("expected cached-guild command registration when the gateway role is enabled and register-all-guilds is set")
	}
}
