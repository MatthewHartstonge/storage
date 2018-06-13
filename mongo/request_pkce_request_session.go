package mongo

import (
	// Standard Library Imports
	"context"

	// External Imports
	"github.com/ory/fosite"
	"github.com/sirupsen/logrus"

	// Internal Imports
	"github.com/matthewhartstonge/storage"
)

func (r *requestMongoManager) CreatePKCERequestSession(ctx context.Context, signature string, request fosite.Requester) error {
	// Initialize contextual method logger
	log := logger.WithFields(logrus.Fields{
		"package":    "mongo",
		"collection": storage.EntityPKCESessions,
		"method":     "CreatePKCERequestSession",
	})

	// Copy a new DB session if none specified
	mgoSession, ok := ContextToMgoSession(ctx)
	if !ok {
		mgoSession = r.db.Session.Copy()
		ctx = MgoSessionToContext(ctx, mgoSession)
		defer mgoSession.Close()
	}

	// Trace how long the Mongo operation takes to complete.
	span, ctx := traceMongoCall(ctx, dbTrace{
		Manager: "requestMongoManager",
		Method:  "CreatePKCERequestSession",
	})
	defer span.Finish()

	// Store session request
	_, err := r.Create(ctx, storage.EntityPKCESessions, toMongo(signature, request))
	if err != nil {
		if err == storage.ErrResourceExists {
			log.WithError(err).Debug(logConflict)
			return err
		}

		// Log to StdOut
		log.WithError(err).Error(logError)
		return err
	}

	return nil
}

func (r *requestMongoManager) GetPKCERequestSession(ctx context.Context, signature string, session fosite.Session) (fosite.Requester, error) {
	// Initialize contextual method logger
	log := logger.WithFields(logrus.Fields{
		"package":    "mongo",
		"collection": storage.EntityPKCESessions,
		"method":     "GetPKCERequestSession",
	})

	// Copy a new DB session if none specified
	mgoSession, ok := ContextToMgoSession(ctx)
	if !ok {
		mgoSession = r.db.Session.Copy()
		ctx = MgoSessionToContext(ctx, mgoSession)
		defer mgoSession.Close()
	}

	// Trace how long the Mongo operation takes to complete.
	span, ctx := traceMongoCall(ctx, dbTrace{
		Manager: "requestMongoManager",
		Method:  "GetPKCERequestSession",
	})
	defer span.Finish()

	// Get the stored request
	req, err := r.GetBySignature(ctx, storage.EntityPKCESessions, signature)
	if err != nil {
		if err == fosite.ErrNotFound {
			log.WithError(err).Debug(logNotFound)
			return nil, err
		}
		// Log to StdOut
		log.WithError(err).Error(logError)
		return nil, err
	}

	// Transform to a fosite.Request
	request, err := req.ToRequest(ctx, session, r.Clients)
	if err != nil {
		if err == fosite.ErrNotFound {
			log.WithError(err).Debug(logNotFound)
			return nil, err
		}
		// Log to StdOut
		log.WithError(err).Error(logError)
		return nil, err
	}

	return request, nil
}

func (r *requestMongoManager) DeletePKCERequestSession(ctx context.Context, signature string) error {
	// Initialize contextual method logger
	log := logger.WithFields(logrus.Fields{
		"package":    "mongo",
		"collection": storage.EntityPKCESessions,
		"method":     "DeletePKCERequestSession",
	})

	// Copy a new DB session if none specified
	mgoSession, ok := ContextToMgoSession(ctx)
	if !ok {
		mgoSession = r.db.Session.Copy()
		ctx = MgoSessionToContext(ctx, mgoSession)
		defer mgoSession.Close()
	}

	// Trace how long the Mongo operation takes to complete.
	span, ctx := traceMongoCall(ctx, dbTrace{
		Manager: "requestMongoManager",
		Method:  "DeletePKCERequestSession",
	})
	defer span.Finish()

	// Remove session request
	err := r.DeleteBySignature(ctx, storage.EntityPKCESessions, signature)
	if err != nil {
		if err == fosite.ErrNotFound {
			log.WithError(err).Debug(logNotFound)
			return err
		}

		// Log to StdOut
		log.WithError(err).Error(logError)
		return err
	}

	return nil
}
