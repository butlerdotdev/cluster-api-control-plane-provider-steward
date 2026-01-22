// Copyright 2025 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package indexers

import (
	"context"

	"github.com/butlerdotdev/steward/indexers"
	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
)

func SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	for _, indexer := range []indexers.Indexer{
		ExternalClusterReferenceStewardControlPlane{},
		ExternalClusterReferenceSecret{},
		StewardControlPlaneUID{},
	} {
		if err := mgr.GetFieldIndexer().IndexField(ctx, indexer.Object(), indexer.Field(), indexer.ExtractValue()); err != nil {
			return errors.Wrap(err, "failed to set up indexer "+indexer.Field())
		}
	}

	return nil
}
