package lib

import (
	"fmt"
	"image/color"
	"regexp"

	"github.com/eliukblau/pixterm/pkg/ansimage"
	"github.com/mrusme/gobbs/ui/ctx"
)

func RenderInlineImages(c *ctx.Ctx, s string, w int) string {
	var re = regexp.MustCompile(`(?m)(http|ftp|https):\/\/([\w_-]+(?:(?:\.[\w_-]+)+))([\w.,@?^=%&:\/~+#-]*[\w@?^=%&\/~+#-])\.(jpg|jpeg|png)`)

	s = re.ReplaceAllStringFunc(s, func(url string) string {
		c.Logger.Debugf("found string: %s\n", url)
		pix, err := ansimage.NewScaledFromURL(
			url,
			int((float32(w) * 0.75)),
			w,
			color.Transparent,
			ansimage.ScaleModeResize,
			ansimage.NoDithering,
		)
		if err != nil {
			c.Logger.Debugf("error: %s\n", err.Error())
			return url
		}

		c.Logger.Debugf("returning rendered image\n")
		return fmt.Sprintf("\n\n%s\nSource: %s\n\n", pix.RenderExt(false, false), url)
	})

	c.Logger.Debugf("returning s\n")
	return s
}
