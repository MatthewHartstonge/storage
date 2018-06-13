package mongo

import (
	// Standard Library imports
	"context"
	"time"

	// External Imports
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/ory/fosite"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"

	// Internal Imports
	"github.com/matthewhartstonge/storage"
)

// clientMongoManager provides a fosite storage implementation for Clients.
//
// Implements:
// - fosite.Storage
// - fosite.ClientManager
// - storage.AuthClientMigrator
// - storage.ClientManager
// - storage.ClientStorer
type clientMongoManager struct {
	db     *mgo.Database
	hasher fosite.Hasher
}

// Configure sets up the Mongo collection for OAuth 2.0 client resources.
func (c *clientMongoManager) Configure(ctx context.Context) error {
	log := logger.WithFields(logrus.Fields{
		"package":    "mongo",
		"collection": storage.EntityClients,
		"method":     "Configure",
	})

	// Copy a new DB session if none specified
	mgoSession, ok := ContextToMgoSession(ctx)
	if !ok {
		mgoSession = c.db.Session.Copy()
		ctx = MgoSessionToContext(ctx, mgoSession)
		defer mgoSession.Close()
	}

	// Build Index
	idxClientId := mgo.Index{
		Name:       IdxClientId,
		Key:        []string{"id"},
		Unique:     true,
		DropDups:   true,
		Background: true,
		Sparse:     true,
	}

	collection := c.db.C(storage.EntityClients).With(mgoSession)
	err := collection.EnsureIndex(idxClientId)
	if err != nil {
		log.WithError(err).Error(logError)
		return err
	}

	return nil
}

// getConcrete returns an OAuth 2.0 Client resource.
func (c *clientMongoManager) getConcrete(ctx context.Context, clientID string) (result storage.Client, err error) {
	log := logger.WithFields(logrus.Fields{
		"package":    "mongo",
		"collection": storage.EntityClients,
		"method":     "getConcrete",
		"id":         clientID,
	})

	// Copy a new DB session if none specified
	mgoSession, ok := ContextToMgoSession(ctx)
	if !ok {
		mgoSession = c.db.Session.Copy()
		ctx = MgoSessionToContext(ctx, mgoSession)
		defer mgoSession.Close()
	}

	// Build Query
	query := bson.M{
		"id": clientID,
	}

	// Trace how long the Mongo operation takes to complete.
	span, ctx := traceMongoCall(ctx, dbTrace{
		Manager: "clientMongoManager",
		Method:  "getConcrete",
		Query:   query,
	})
	defer span.Finish()

	storageClient := storage.Client{}
	collection := c.db.C(storage.EntityClients).With(mgoSession)
	if err := collection.Find(query).One(&storageClient); err != nil {
		if err == mgo.ErrNotFound {
			log.WithError(err).Debug(logNotFound)
			return result, fosite.ErrNotFound
		}

		// Log to StdOut
		log.WithError(err).Error(logError)
		// Log to OpenTracing
		otLogErr(span, err)
		return result, err
	}
	return storageClient, nil
}

// List filters resources to return a list of OAuth 2.0 client resources.
func (c *clientMongoManager) List(ctx context.Context, filter storage.ListClientsRequest) (results []storage.Client, err error) {
	// Initialize contextual method logger
	log := logger.WithFields(logrus.Fields{
		"package":    "mongo",
		"collection": storage.EntityClients,
		"method":     "List",
	})

	// Copy a new DB session if none specified
	mgoSession, ok := ContextToMgoSession(ctx)
	if !ok {
		mgoSession = c.db.Session.Copy()
		ctx = MgoSessionToContext(ctx, mgoSession)
		defer mgoSession.Close()
	}

	// Build Query
	query := bson.M{}
	if filter.AllowedTenantAccess != "" {
		query["allowedTenantAccess"] = filter.AllowedTenantAccess
	}
	if filter.RedirectURI != "" {
		query["redirectUris"] = filter.RedirectURI
	}
	if filter.GrantType != "" {
		query["grantTypes"] = filter.GrantType
	}
	if filter.ResponseType != "" {
		query["responseTypes"] = filter.ResponseType
	}
	if len(filter.ScopesIntersection) > 0 {
		query["scopes"] = bson.M{"$all": filter.ScopesIntersection}
	}
	if len(filter.ScopesUnion) > 0 {
		query["scopes"] = bson.M{"$in": filter.ScopesUnion}
	}
	if filter.Contact != "" {
		query["contacts"] = filter.Contact
	}
	if filter.Public {
		query["public"] = filter.Public
	}
	if filter.Disabled {
		query["disabled"] = filter.Disabled
	}

	// Trace how long the Mongo operation takes to complete.
	span, ctx := traceMongoCall(ctx, dbTrace{
		Manager: "clientMongoManager",
		Method:  "List",
		Query:   query,
	})
	defer span.Finish()

	var clients []storage.Client
	collection := c.db.C(storage.EntityClients).With(mgoSession)
	err = collection.Find(query).All(&clients)
	if err != nil {
		// Log to StdOut
		log.WithError(err).Error(logError)
		// Log to OpenTracing
		otLogErr(span, err)
		return results, err
	}
	return clients, nil
}

// Create stores a new OAuth2.0 Client resource.
func (c *clientMongoManager) Create(ctx context.Context, client storage.Client) (result storage.Client, err error) {
	// Initialize contextual method logger
	log := logger.WithFields(logrus.Fields{
		"package":    "mongo",
		"collection": storage.EntityClients,
		"method":     "Create",
	})

	// Copy a new DB session if none specified
	mgoSession, ok := ContextToMgoSession(ctx)
	if !ok {
		mgoSession = c.db.Session.Copy()
		ctx = MgoSessionToContext(ctx, mgoSession)
		defer mgoSession.Close()
	}

	// Enable developers to provide their own IDs
	if client.ID == "" {
		client.ID = uuid.New()
	}
	if client.CreateTime == 0 {
		client.CreateTime = time.Now().Unix()
	}

	// Hash incoming secret
	hash, err := c.hasher.Hash([]byte(client.Secret))
	if err != nil {
		log.WithError(err).Error(logNotHashable)
		return result, err
	}
	client.Secret = string(hash)

	// Trace how long the Mongo operation takes to complete.
	span, ctx := traceMongoCall(ctx, dbTrace{
		Manager: "clientMongoManager",
		Method:  "Create",
	})
	defer span.Finish()

	// Create resource
	collection := c.db.C(storage.EntityClients).With(mgoSession)
	err = collection.Insert(client)
	if err != nil {
		if mgo.IsDup(err) {
			// Log to StdOut
			log.WithError(err).Debug(logConflict)
			// Log to OpenTracing
			otLogErr(span, err)
			return result, storage.ErrResourceExists
		}

		// Log to StdOut
		log.WithError(err).Error(logError)
		// Log to OpenTracing
		client.Secret = "REDACTED"
		otLogQuery(span, client)
		otLogErr(span, err)
		return result, err
	}
	return client, nil
}

// Get finds and returns an OAuth 2.0 client resource.
func (c *clientMongoManager) Get(ctx context.Context, clientID string) (result storage.Client, err error) {
	return c.getConcrete(ctx, clientID)
}

// GetClient finds and returns an OAuth 2.0 client resource.
//
// GetClient implements:
// - fosite.Storage
// - fosite.ClientManager
func (c *clientMongoManager) GetClient(ctx context.Context, clientID string) (fosite.Client, error) {
	client, err := c.getConcrete(ctx, clientID)
	if err != nil {
		return nil, err
	}
	return &client, nil
}

// Update updates an OAuth 2.0 client resource.
func (c *clientMongoManager) Update(ctx context.Context, clientID string, updatedClient storage.Client) (result storage.Client, err error) {
	// Initialize contextual method logger
	log := logger.WithFields(logrus.Fields{
		"package":    "mongo",
		"collection": storage.EntityClients,
		"method":     "Update",
		"id":         clientID,
	})

	// Copy a new DB session if none specified
	mgoSession, ok := ContextToMgoSession(ctx)
	if !ok {
		mgoSession = c.db.Session.Copy()
		ctx = MgoSessionToContext(ctx, mgoSession)
		defer mgoSession.Close()
	}

	currentResource, err := c.getConcrete(ctx, clientID)
	if err != nil {
		if err == fosite.ErrNotFound {
			log.Debug(logNotFound)
			return result, err
		}

		log.WithError(err).Error(logError)
		return result, err
	}

	// Deny updating the entity Id
	updatedClient.ID = clientID
	// Update modified time
	updatedClient.UpdateTime = time.Now().Unix()

	if currentResource.Secret == updatedClient.Secret || updatedClient.Secret == "" {
		// If the password/hash is blank or hash matches, set using old hash.
		updatedClient.Secret = currentResource.Secret
	} else {
		newHash, err := c.hasher.Hash([]byte(updatedClient.Secret))
		if err != nil {
			log.WithError(err).Error(logNotHashable)
			return result, err
		}
		updatedClient.Secret = string(newHash)
	}

	// Build Query
	selector := bson.M{
		"id": clientID,
	}

	// Trace how long the Mongo operation takes to complete.
	span, ctx := traceMongoCall(ctx, dbTrace{
		Manager:  "clientMongoManager",
		Method:   "Update",
		Selector: selector,
	})
	defer span.Finish()

	collection := c.db.C(storage.EntityClients).With(mgoSession)
	if err := collection.Update(selector, updatedClient); err != nil {
		if err == mgo.ErrNotFound {
			// Log to StdOut
			log.WithError(err).Debug(logNotFound)
			// Log to OpenTracing
			otLogErr(span, err)
			return result, fosite.ErrNotFound
		}

		// Log to StdOut
		log.WithError(err).Error(logError)
		// Log to OpenTracing
		otLogQuery(span, updatedClient)
		otLogErr(span, err)
		return result, err
	}
	return updatedClient, nil
}

// Migrate is provided solely for the case where you want to migrate clients and
// upgrade their password using the AuthClientMigrator interface.
// This performs an upsert, either creating or overwriting the record with the
// newly provided full record. Use with caution, be secure, don't be dumb.
func (u *clientMongoManager) Migrate(ctx context.Context, migratedClient storage.Client) (result storage.Client, err error) {
	// Initialize contextual method logger
	log := logger.WithFields(logrus.Fields{
		"package":    "mongo",
		"collection": storage.EntityClients,
		"method":     "Migrate",
	})

	// Copy a new DB session if none specified
	mgoSession, ok := ContextToMgoSession(ctx)
	if !ok {
		mgoSession = u.db.Session.Copy()
		ctx = MgoSessionToContext(ctx, mgoSession)
		defer mgoSession.Close()
	}

	// Generate a unique ID if not supplied
	if migratedClient.ID == "" {
		migratedClient.ID = uuid.New()
	}
	// Update create time
	if migratedClient.CreateTime == 0 {
		migratedClient.CreateTime = time.Now().Unix()
	} else {
		// Update modified time
		migratedClient.UpdateTime = time.Now().Unix()
	}

	// Build Query
	selector := bson.M{
		"id": migratedClient.ID,
	}

	// Trace how long the Mongo operation takes to complete.
	span, ctx := traceMongoCall(ctx, dbTrace{
		Manager:  "clientMongoManager",
		Method:   "Migrate",
		Selector: selector,
	})
	defer span.Finish()

	collection := u.db.C(storage.EntityClients).With(mgoSession)
	if _, err := collection.Upsert(selector, migratedClient); err != nil {
		if err == mgo.ErrNotFound {
			// Log to StdOut
			log.WithError(err).Debug(logNotFound)
			// Log to OpenTracing
			otLogErr(span, err)
			return result, fosite.ErrNotFound
		}

		if mgo.IsDup(err) {
			// Log to StdOut
			log.WithError(err).Debug(logConflict)
			// Log to OpenTracing
			otLogErr(span, err)
			return result, storage.ErrResourceExists
		}

		// Log to StdOut
		log.WithError(err).Error(logError)
		// Log to OpenTracing
		otLogQuery(span, migratedClient)
		otLogErr(span, err)
		return result, err
	}
	return migratedClient, nil
}

// Delete removes an OAuth 2.0 Client resource.
func (c *clientMongoManager) Delete(ctx context.Context, clientID string) error {
	// Initialize contextual method logger
	log := logger.WithFields(logrus.Fields{
		"package":    "mongo",
		"collection": storage.EntityClients,
		"method":     "Delete",
		"id":         clientID,
	})

	// Copy a new DB session if none specified
	mgoSession, ok := ContextToMgoSession(ctx)
	if !ok {
		mgoSession = c.db.Session.Copy()
		ctx = MgoSessionToContext(ctx, mgoSession)
		defer mgoSession.Close()
	}

	// Build Query
	query := bson.M{
		"id": clientID,
	}

	// Trace how long the Mongo operation takes to complete.
	span, ctx := traceMongoCall(ctx, dbTrace{
		Manager: "clientMongoManager",
		Method:  "Delete",
		Query:   query,
	})
	defer span.Finish()

	collection := c.db.C(storage.EntityClients).With(mgoSession)
	if err := collection.Remove(query); err != nil {
		if err == mgo.ErrNotFound {
			// Log to StdOut
			log.WithError(err).Debug(logNotFound)
			// Log to OpenTracing
			otLogErr(span, err)
			return fosite.ErrNotFound
		}

		// Log to StdOut
		log.WithError(err).Error(logError)
		// Log to OpenTracing
		otLogErr(span, err)
		return err
	}
	return nil
}

// Authenticate verifies the identity of a client resource.
func (c *clientMongoManager) Authenticate(ctx context.Context, clientID string, secret string) (result storage.Client, err error) {
	// Initialize contextual method logger
	log := logger.WithFields(logrus.Fields{
		"package":    "mongo",
		"collection": storage.EntityClients,
		"method":     "Authenticate",
		"id":         clientID,
	})

	// Copy a new DB session if none specified
	mgoSession, ok := ContextToMgoSession(ctx)
	if !ok {
		mgoSession = c.db.Session.Copy()
		ctx = MgoSessionToContext(ctx, mgoSession)
		defer mgoSession.Close()
	}

	// Trace how long the Mongo operation takes to complete.
	span, ctx := traceMongoCall(ctx, dbTrace{
		Manager: "clientMongoManager",
		Method:  "Authenticate",
	})
	defer span.Finish()

	client, err := c.getConcrete(ctx, clientID)
	if err != nil {
		if err == fosite.ErrNotFound {
			log.Debug(logNotFound)
			return result, err
		}

		log.WithError(err).Error(logError)
		return result, err
	}

	if client.Public {
		// The client doesn't have a secret, therefore is authenticated
		// implicitly.
		log.Debug("public client allowed access")
		return client, nil
	}

	if client.Disabled {
		log.Debug("disabled client denied access")
		return result, fosite.ErrAccessDenied
	}

	err = c.hasher.Compare(client.GetHashedSecret(), []byte(secret))
	if err != nil {
		log.WithError(err).Warn("failed to authenticate client secret")
		return result, err
	}

	return client, nil
}

func (c *clientMongoManager) AuthenticateMigration(ctx context.Context, currentAuth storage.AuthClientFunc, clientID string, secret string) (result storage.Client, err error) {
	// Initialize contextual method logger
	log := logger.WithFields(logrus.Fields{
		"package":    "mongo",
		"collection": storage.EntityClients,
		"method":     "AuthenticateMigration",
		"id":         clientID,
	})

	// Copy a new DB session if none specified
	mgoSession, ok := ContextToMgoSession(ctx)
	if !ok {
		mgoSession = c.db.Session.Copy()
		ctx = MgoSessionToContext(ctx, mgoSession)
		defer mgoSession.Close()
	}

	// Trace how long the Mongo operation takes to complete.
	span, ctx := traceMongoCall(ctx, dbTrace{
		Manager: "clientMongoManager",
		Method:  "AuthenticateMigration",
	})
	defer span.Finish()

	// Authenticate with old hasher
	client, authenticated := currentAuth()

	// Check for client not found
	if client.IsEmpty() && !authenticated {
		log.Debug(logNotFound)
		return result, fosite.ErrNotFound
	}

	if client.Public {
		// The client doesn't have a secret, therefore is authenticated
		// implicitly.
		log.Debug("public client allowed access")
		return client, nil
	}

	if client.Disabled {
		log.Debug("disabled client denied access")
		return result, fosite.ErrAccessDenied
	}

	if !authenticated {
		// If client isn't authenticated, try authenticating with new hasher.
		err := c.hasher.Compare(client.GetHashedSecret(), []byte(secret))
		if err != nil {
			log.WithError(err).Warn("failed to authenticate client secret")
			return result, err
		}
		return client, nil
	}

	// If the client is found and authenticated, create a new hash using the new
	// hasher, update the database record and return the record with no error.
	newHash, err := c.hasher.Hash([]byte(secret))
	if err != nil {
		log.WithError(err).Error(logNotHashable)
		return result, err
	}

	// Save the new hash
	client.Secret = string(newHash)
	return c.Update(ctx, clientID, client)
}

func (c *clientMongoManager) GrantScopes(ctx context.Context, clientID string, scopes []string) (result storage.Client, err error) {
	// Initialize contextual method logger
	log := logger.WithFields(logrus.Fields{
		"package":    "mongo",
		"collection": storage.EntityClients,
		"method":     "GrantScopes",
		"id":         clientID,
	})

	// Copy a new DB session if none specified
	mgoSession, ok := ContextToMgoSession(ctx)
	if !ok {
		mgoSession = c.db.Session.Copy()
		ctx = MgoSessionToContext(ctx, mgoSession)
		defer mgoSession.Close()
	}

	// Trace how long the Mongo operation takes to complete.
	span, ctx := traceMongoCall(ctx, dbTrace{
		Manager: "clientMongoManager",
		Method:  "GrantScopes",
	})
	defer span.Finish()

	client, err := c.getConcrete(ctx, clientID)
	if err != nil {
		if err == fosite.ErrNotFound {
			log.Debug(logNotFound)
			return result, err
		}

		log.WithError(err).Error(logError)
		return result, err
	}

	client.EnableScopeAccess(scopes...)
	return c.Update(ctx, client.ID, client)
}

func (c *clientMongoManager) RemoveScopes(ctx context.Context, clientID string, scopes []string) (result storage.Client, err error) {
	// Initialize contextual method logger
	log := logger.WithFields(logrus.Fields{
		"package":    "mongo",
		"collection": storage.EntityClients,
		"method":     "RemoveScopes",
		"id":         clientID,
	})

	// Copy a new DB session if none specified
	mgoSession, ok := ContextToMgoSession(ctx)
	if !ok {
		mgoSession = c.db.Session.Copy()
		ctx = MgoSessionToContext(ctx, mgoSession)
		defer mgoSession.Close()
	}

	// Trace how long the Mongo operation takes to complete.
	span, ctx := traceMongoCall(ctx, dbTrace{
		Manager: "clientMongoManager",
		Method:  "RemoveScopes",
	})
	defer span.Finish()

	client, err := c.getConcrete(ctx, clientID)
	if err != nil {
		if err == fosite.ErrNotFound {
			log.Debug(logNotFound)
			return result, err
		}

		log.WithError(err).Error(logError)
		return result, err
	}

	client.DisableScopeAccess(scopes...)
	return c.Update(ctx, client.ID, client)
}
