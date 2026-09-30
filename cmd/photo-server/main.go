// photo-server streams a random photo from a directory to photo displays
// (photo-gui), moving on to another every PHOTO_SECONDS. Every photo is shown
// once, in a random order, before any repeats.
package main

import (
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	photopb "homeserver/gen/photo"
	"homeserver/internal/broadcast"
)

// version is set at build time (-ldflags "-X main.version=...") and sent to
// clients as "server-version" header metadata on every stream.
var version = "dev"

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	addr := envOr("LISTEN_ADDR", ":9092")
	dir := envOr("PHOTOS_DIR", "/photos")
	seconds, err := strconv.Atoi(envOr("PHOTO_SECONDS", "60"))
	if err != nil || seconds <= 0 {
		log.Fatalf("PHOTO_SECONDS must be a positive number of seconds, got %q", os.Getenv("PHOTO_SECONDS"))
	}

	bc := broadcast.New[*pick]()
	go pickPhotos(newLibrary(dir), time.Duration(seconds)*time.Second, bc)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen on %s: %v", addr, err)
	}
	srv := grpc.NewServer(grpc.StreamInterceptor(func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := ss.SetHeader(metadata.Pairs("server-version", version)); err != nil {
			return err
		}
		return handler(srv, ss)
	}))
	photopb.RegisterPhotoServiceServer(srv, &photoServer{bc: bc})

	log.Printf("photo-server %s listening on %s, photos from %s", version, addr, dir)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

type photoServer struct {
	photopb.UnimplementedPhotoServiceServer
	bc *broadcast.Broadcaster[*pick]
}

func (s *photoServer) StreamPhotos(req *photopb.StreamPhotosRequest, stream photopb.PhotoService_StreamPhotosServer) error {
	w, h := int(max(req.Width, 0)), int(max(req.Height, 0))
	ch := s.bc.Subscribe()
	defer s.bc.Unsubscribe(ch)
	for {
		select {
		case p := <-ch:
			data, err := encodeFit(p.img, w, h)
			if err != nil {
				log.Printf("encoding %s: %v", p.name, err)
				continue
			}
			if err := stream.Send(&photopb.Photo{Jpeg: data, Name: p.name, SinceUnix: p.since.Unix(), NextUnix: p.next.Unix()}); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return nil
		}
	}
}
