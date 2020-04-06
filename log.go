package blathers

import (
	"context"
	"fmt"
	"log"
)

func writeLog(ctx context.Context, l string) {
	log.Printf("%s: %s%s", RequestID(ctx), DebuggingPrefix(ctx), l)
}

func writeLogf(ctx context.Context, l string, f ...interface{}) {
	log.Printf("%s: %s%s", RequestID(ctx), DebuggingPrefix(ctx), fmt.Sprintf(l, f...))
}

func wrap(ctx context.Context, err error, wrap string) error {
	return fmt.Errorf(
		"%s: %s%s: %s",
		RequestID(ctx),
		DebuggingPrefix(ctx),
		wrap,
		err.Error(),
	)
}

func wrapf(ctx context.Context, err error, wrap string, f ...interface{}) error {
	return fmt.Errorf(
		"%s: %s%s: %s",
		RequestID(ctx),
		DebuggingPrefix(ctx),
		fmt.Sprintf(wrap, f...),
		err.Error(),
	)
}
