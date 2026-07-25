package appfx

import (
	"context"
	"gig-service/internal/domain"
	usergrpc "gig-service/internal/infra/user/grpc"
	"github.com/ofm-microservices/ofm-common/pkg/logging"
	"go.uber.org/fx"
)

// InvokeWarmupSellerLookups backfills missing seller usernames and seeds the
// local username lookup cache on startup.
func InvokeWarmupSellerLookups(
	lc fx.Lifecycle,
	repo domain.GigRepository,
	readRepo domain.GigReadRepository,
	users usergrpc.UserPreviewClient,
	lg logging.Logger,
) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				ctx := context.WithoutCancel(context.Background())
				gigs, err := repo.ListAll(ctx)
				if err != nil {
					lg.Error("gig warmup load failed", logging.Operation("gig.bootstrap.warmup"), logging.Err(err))
					return
				}
				for _, gig := range gigs {
					if gig == nil {
						continue
					}
					username := gig.SellerUsername
					if username == "" && users != nil {
						preview, err := users.GetUserPreviewByIDNoCache(ctx, gig.FreelancerID)
						if err != nil {
							lg.Error("gig seller username backfill failed",
								logging.Operation("gig.bootstrap.warmup"),
								logging.String("gig_id", gig.ID),
								logging.String("user_id", gig.FreelancerID),
								logging.Err(err),
							)
							continue
						}
						username = preview.Username
						if username != "" {
							if _, err := repo.UpdateSellerUsername(ctx, gig.ID, username); err != nil {
								lg.Error("gig seller username persist failed",
									logging.Operation("gig.bootstrap.warmup"),
									logging.String("gig_id", gig.ID),
									logging.String("user_id", gig.FreelancerID),
									logging.String("username", username),
									logging.Err(err),
								)
							}
						}
					}
					if username == "" {
						continue
					}
					if err := readRepo.SetUserLookup(ctx, username, gig.FreelancerID, 0); err != nil {
						lg.Error("gig username lookup warmup failed",
							logging.Operation("gig.bootstrap.warmup"),
							logging.String("username", username),
							logging.String("user_id", gig.FreelancerID),
							logging.Err(err),
						)
					}
				}
			}()
			return nil
		},
		OnStop: func(context.Context) error { return nil },
	})
}
