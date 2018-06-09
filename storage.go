package storage

import "context"

// AuthFunc enables developers to supply their own authentication function,
// to check old hashes that need to be upgraded.
//
// For example, you may have passwords in MD5 and wanting them to be
// migrated to fosite's default hasher, bcrypt. Therefore, if you do a mass
// data migration, the function you supply would have to:
//   +- Shortcut logic if the hash string prefix matches what you expect from the new hash
//   - Get the current client record (return nil, false if not found)
//   - Authenticate the current DB secret against MD5
//   - Return the Client record and if the client authenticated.
//   - if true, the AuthenticateMigration will upgrade the hash.
type AuthFunc func() (Client, bool)

// AuthMigrator provides an interface to enable storage backends to implement
// functionality to upgrade hashes currently stored in the datastore.
type AuthMigrator interface {
	// AuthenticateMigration enables developers to supply your own
	// authentication function, which in turn, if true, will migrate the secret
	// to the hasher implemented within fosite.
	AuthenticateMigration(ctx context.Context, currentAuth AuthFunc, id string, secret []byte) (Client, error)
}

// Configurer enables an implementer to configure required migrations, indexing
// and is called when the datastore connects.
type Configurer interface {
	// Configure configures the underlying database engine to match
	// requirements.
	// Configure will be called each time a service is started, so ensure this
	// function maintains idempotency.
	//The main use here is to apply creation of tables, collections, schemas
	// any needed migrations and configuration of indexes as required.
	Configure()
}