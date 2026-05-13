package daemon

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jessevdk/go-flags"

	"github.com/canonical/workshop/internal/overlord/hookstate"
	"github.com/canonical/workshop/internal/overlord/hookstate/ctlcmd"
	"github.com/canonical/workshop/internal/secrets"
	secretsbuiltin "github.com/canonical/workshop/internal/secrets/builtin"
	"github.com/canonical/workshop/internal/workshop"
)

// workshopCtlOptions holds the various options with which workshopctl is invoked.
type workshopCtlOptions struct {
	// ContextID is a string used to determine the context of this call (e.g.
	// which context and handler should be used, etc.)
	ContextID string `json:"context-id"`

	// Args contains a list of parameters to use for this invocation.
	Args []string `json:"args"`
}

// workshopCtlPostData is the data posted to the daemon /v2/workshopctl endpoint
// TODO: this can be removed once we no longer need to pass stdin data
// but instead use a real stdin stream
type workshopCtlPostData struct {
	workshopCtlOptions

	Stdin []byte `json:"stdin,omitempty"`
}

type workshopctlOutput struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

func v1PostWorkshopCtl(c *Command, r *http.Request, _ *userState) Response {
	var reqData workshopCtlPostData

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reqData); err != nil {
		return statusBadRequest("cannot decode data from request body: %w", err)
	}

	_, uid, _, err := ucrednetGet(r.RemoteAddr)
	if err != nil {
		return statusForbidden("cannot get remote user: %w", err)
	}

	// Ignore missing context error to allow 'workshopctl -h' without a context;
	// Actual context is validated later by get/set.
	context, _ := c.d.overlord.HookManager().Context(reqData.ContextID)

	if reqData.Stdin != nil {
		context.Lock()
		context.Set("stdin", reqData.Stdin)
		context.Unlock()
	}

	if context != nil {
		if err := injectSecretResolver(c, context); err != nil {
			return statusInternalError(
				"cannot prepare secret resolver: %w", err,
			)
		}
	}

	stdout, stderr, err := ctlcmd.Run(context, reqData.Args, uid)
	if err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			stdout = []byte(e.Error())
		} else {
			return statusBadRequest("%w", err)
		}
	}

	result := workshopctlOutput{
		Stdout: string(stdout),
		Stderr: string(stderr),
	}

	return SyncResponse(result, http.StatusOK)
}

// injectSecretResolver caches a secret resolver onto the context so
// workshopctl get-secret can use it. For hook contexts it binds to the
// hook's SDK; for long-lived workshop-cookie contexts it caches a
// workshop-scoped resolver (SDK supplied at call time).
func injectSecretResolver(
	c *Command, ctx *hookstate.Context,
) error {
	ctx.Lock()
	defer ctx.Unlock()

	repo := c.d.overlord.InterfaceManager().Repository()
	resolver := secrets.NewResolver(repo, getSecretProvider)

	if task, ok := ctx.Task(); ok {
		var prj workshop.Project
		if err := task.Get("project", &prj); err != nil {
			return err
		}

		var ws string
		if err := task.Get("workshop", &ws); err != nil {
			return err
		}

		var owner string
		if err := task.Change().Get("user", &owner); err != nil {
			return err
		}

		sdkName := ctx.Sdk()
		if sdkName == "" {
			return nil
		}

		bound := resolver.Bind(prj.ProjectId, ws, sdkName)
		// Inject the workshop owner onto every resolver invocation so
		// providers like secret-service can reach the owner's session
		// bus or keychain. The resolver itself stays oblivious to
		// workshop state; the wiring layer owns that concern.
		ctx.Cache("secret-resolver", secrets.SecretResolverFunc(
			func(rctx context.Context, plugName string) (string, error) {
				rctx = context.WithValue(rctx, workshop.ContextUser, owner)
				return bound(rctx, plugName)
			},
		))
		return nil
	}

	cookie := ctx.Cookie()
	if cookie == nil {
		return nil
	}
	prj := cookie.Project
	ws := cookie.Workshop
	owner := cookie.User
	ctx.Cache("workshop-secret-resolver", ctlcmd.WorkshopSecretResolverFunc(
		func(rctx context.Context, sdkName, plugName string) (string, error) {
			rctx = context.WithValue(rctx, workshop.ContextUser, owner)
			return resolver.ResolvePlug(rctx, prj.ProjectId, ws, sdkName, plugName)
		},
	))
	return nil
}

// getSecretProvider looks up a secret provider by name in the builtin
// registry. It is declared as a variable so tests can swap it for a
// fake without touching the global registry.
var getSecretProvider secrets.ProviderLookup = secretsbuiltin.GetProvider
