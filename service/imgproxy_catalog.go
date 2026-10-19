package service

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/vvb13a/godam/domain"
)

// CatalogArgKind tells the Vue configurator which input to render and tells the
// backend how to validate a preset value.
type CatalogArgKind string

const (
	ArgInt    CatalogArgKind = "int"
	ArgFloat  CatalogArgKind = "float"
	ArgString CatalogArgKind = "string"
	ArgBool   CatalogArgKind = "bool"
	ArgEnum   CatalogArgKind = "enum"
	ArgColor  CatalogArgKind = "color"
	ArgBase64 CatalogArgKind = "base64"
)

type CatalogChoice struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type CatalogArg struct {
	Name        string          `json:"name"`
	Label       string          `json:"label"`
	Kind        CatalogArgKind  `json:"kind"`
	Required    bool            `json:"required,omitempty"`
	Default     string          `json:"default,omitempty"`
	Options     []CatalogChoice `json:"options,omitempty"`
	Min         *float64        `json:"min,omitempty"`
	Max         *float64        `json:"max,omitempty"`
	Placeholder string          `json:"placeholder,omitempty"`
	Help        string          `json:"help,omitempty"`
}

// CatalogOption is one imgproxy processing option exposed by the configurator.
// Raw options have no structured arguments and are edited as colon-separated
// values (useful for the long, Pro-only option families).
type CatalogOption struct {
	Name        string       `json:"name"`
	Label       string       `json:"label"`
	Category    string       `json:"category"`
	Description string       `json:"description"`
	Pro         bool         `json:"pro,omitempty"`
	Raw         bool         `json:"raw,omitempty"`
	Args        []CatalogArg `json:"args,omitempty"`
}

type Catalog struct {
	Formats []string        `json:"formats"`
	Options []CatalogOption `json:"options"`
}

var catalogOptions = []CatalogOption{
	// ---- Resize -----------------------------------------------------------
	opt("resize", "Resize", "Resize", "Resize with a resizing type, width, height, enlarge and extend.", false,
		enumArg("resizing_type", "Resizing type", "fit", true,
			choice("fit", "Fit"), choice("fill", "Fill"), choice("fill-down", "Fill down"), choice("force", "Force"), choice("auto", "Auto")),
		intArg("width", "Width", "0", 0, 4096, false),
		intArg("height", "Height", "0", 0, 4096, false),
		boolArg("enlarge", "Enlarge", "0"),
		boolArg("extend", "Extend", "0"),
	),
	opt("size", "Size", "Resize", "Resize by width/height, enlarge and extend.", false,
		intArg("width", "Width", "0", 0, 4096, false),
		intArg("height", "Height", "0", 0, 4096, false),
		boolArg("enlarge", "Enlarge", "0"),
		boolArg("extend", "Extend", "0"),
	),
	opt("resizing_type", "Resizing type", "Resize", "How imgproxy resizes the image.", false,
		enumArg("type", "Type", "fit", true,
			choice("fit", "Fit"), choice("fill", "Fill"), choice("fill-down", "Fill down"), choice("force", "Force"), choice("auto", "Auto")),
	),
	opt("resizing_algorithm", "Resizing algorithm", "Resize", "Resampling algorithm (imgproxy Pro).", true,
		enumArg("algorithm", "Algorithm", "lanczos3", true,
			choice("nearest", "Nearest"), choice("linear", "Linear"), choice("cubic", "Cubic"),
			choice("lanczos2", "Lanczos 2"), choice("lanczos3", "Lanczos 3"))),
	opt("width", "Width", "Resize", "Resulting width in pixels (0 keeps the aspect ratio).", false,
		intArg("width", "Width", "0", 0, 4096, false)),
	opt("height", "Height", "Resize", "Resulting height in pixels (0 keeps the aspect ratio).", false,
		intArg("height", "Height", "0", 0, 4096, false)),
	opt("min_width", "Min width", "Resize", "Minimum resulting width.", false,
		intArg("width", "Width", "0", 0, 4096, false)),
	opt("min_height", "Min height", "Resize", "Minimum resulting height.", false,
		intArg("height", "Height", "0", 0, 4096, false)),
	opt("zoom", "Zoom", "Resize", "Multiply the calculated size by these factors.", false,
		floatArg("zoom_x", "Zoom X", "1", 0, 16, false),
		floatArg("zoom_y", "Zoom Y", "1", 0, 16, false),
	),
	opt("dpr", "DPR", "Resize", "Multiply dimensions for HiDPI output.", false,
		floatArg("dpr", "DPR", "1", 0.01, 16, false)),
	opt("enlarge", "Enlarge", "Resize", "Allow the image to be enlarged.", false,
		boolArg("enlarge", "Enlarge", "1")),
	opt("extend", "Extend", "Resize", "Extend the image to the requested size.", false,
		boolArg("extend", "Extend", "1"),
		enumArg("gravity", "Gravity", "ce", false, gravityChoices()...)),
	opt("extend_aspect_ratio", "Extend aspect ratio", "Resize", "Extend the image to the requested aspect ratio.", false,
		boolArg("extend", "Extend", "1"),
		enumArg("gravity", "Gravity", "ce", false, gravityChoices()...)),

	// ---- Crop & orientation ----------------------------------------------
	opt("gravity", "Gravity", "Crop & orientation", "Where imgproxy crops from when resizing.", false,
		enumArg("type", "Type", "ce", true, gravityChoices()...),
		stringArg("x_offset", "X offset", "", "0.5 or 100"),
		stringArg("y_offset", "Y offset", "", "0.5 or 100"),
	),
	opt("crop", "Crop", "Crop & orientation", "Crop an area before resizing.", false,
		intArg("width", "Width", "0", 0, 4096, false),
		intArg("height", "Height", "0", 0, 4096, false),
		enumArg("gravity", "Gravity", "ce", false, gravityChoices()...),
	),
	opt("crop_aspect_ratio", "Crop aspect ratio", "Crop & orientation", "Correct the crop area aspect ratio (imgproxy Pro).", true,
		floatArg("aspect_ratio", "Aspect ratio", "1", 0, 100, false),
		boolArg("enlarge", "Enlarge", "0")),
	opt("objects_position", "Objects position", "Crop & orientation", "Position detected objects in the result (imgproxy Pro).", true),
	opt("trim", "Trim", "Crop & orientation", "Remove surrounding background.", false,
		floatArg("threshold", "Threshold", "10", 0, 255, false),
		colorArg("color", "Background color", ""),
		boolArg("equal_hor", "Equal horizontal", "0"),
		boolArg("equal_ver", "Equal vertical", "0"),
	),
	opt("padding", "Padding", "Crop & orientation", "Add CSS-style padding around the result.", false,
		intArg("top", "Top", "0", 0, 4096, false),
		intArg("right", "Right", "0", 0, 4096, false),
		intArg("bottom", "Bottom", "0", 0, 4096, false),
		intArg("left", "Left", "0", 0, 4096, false),
	),
	opt("auto_rotate", "Auto rotate", "Crop & orientation", "Rotate using the EXIF orientation.", false,
		boolArg("auto_rotate", "Auto rotate", "1")),
	opt("rotate", "Rotate", "Crop & orientation", "Rotate clockwise in 90 degree steps.", false,
		enumArg("angle", "Angle", "90", true,
			choice("0", "0°"), choice("90", "90°"), choice("180", "180°"), choice("270", "270°"))),
	opt("flip", "Flip", "Crop & orientation", "Mirror horizontally and/or vertically.", false,
		boolArg("horizontal", "Horizontal", "1"),
		boolArg("vertical", "Vertical", "0")),

	// ---- Format & quality -------------------------------------------------
	opt("quality", "Quality", "Format & quality", "Output quality percentage.", false,
		intArg("quality", "Quality", "80", 0, 100, false)),
	opt("format_quality", "Format quality", "Format & quality", "Per-format quality pairs, e.g. jpeg:70:webp:60.", false),
	opt("max_bytes", "Max bytes", "Format & quality", "Reduce quality until the result fits this size.", false,
		intArg("bytes", "Max bytes", "0", 0, 1<<30, false)),
	opt("autoquality", "Autoquality", "Format & quality", "Pick quality automatically (imgproxy Pro).", true,
		stringArg("method", "Method", "none", "dssim, ml, none"),
		floatArg("target", "Target", "0.02", 0, 1, false),
		intArg("min_quality", "Min quality", "70", 0, 100, false),
		intArg("max_quality", "Max quality", "80", 0, 100, false),
		floatArg("allowed_error", "Allowed error", "0.002", 0, 1, false),
	),

	// ---- Adjustments ------------------------------------------------------
	opt("adjust", "Adjust", "Adjustments", "Brightness, contrast and saturation (imgproxy Pro).", true,
		intArg("brightness", "Brightness", "0", -255, 255, false),
		floatArg("contrast", "Contrast", "1", 0, 10, false),
		floatArg("saturation", "Saturation", "1", 0, 10, false),
	),
	opt("brightness", "Brightness", "Adjustments", "Adjust brightness (imgproxy Pro).", true,
		intArg("brightness", "Brightness", "0", -255, 255, false)),
	opt("contrast", "Contrast", "Adjustments", "Adjust contrast (imgproxy Pro).", true,
		floatArg("contrast", "Contrast", "1", 0, 10, false)),
	opt("saturation", "Saturation", "Adjustments", "Adjust saturation (imgproxy Pro).", true,
		floatArg("saturation", "Saturation", "1", 0, 10, false)),
	opt("monochrome", "Monochrome", "Adjustments", "Convert to monochrome (imgproxy Pro).", true,
		floatArg("intensity", "Intensity", "1", 0, 1, false),
		colorArg("color", "Base color", "b3b3b3")),
	opt("duotone", "Duotone", "Adjustments", "Convert to a two-color palette (imgproxy Pro).", true,
		floatArg("intensity", "Intensity", "1", 0, 1, false),
		colorArg("color1", "Dark color", "000000"),
		colorArg("color2", "Light color", "ffffff")),
	opt("blur", "Blur", "Adjustments", "Gaussian blur.", false,
		floatArg("sigma", "Sigma", "1", 0, 100, false)),
	opt("sharpen", "Sharpen", "Adjustments", "Sharpen the image.", false,
		floatArg("sigma", "Sigma", "0.5", 0, 100, false)),
	opt("pixelate", "Pixelate", "Adjustments", "Pixelate the image.", false,
		intArg("size", "Pixel size", "10", 1, 1000, false)),
	opt("unsharp_masking", "Unsharp masking", "Adjustments", "Controlled sharpening (imgproxy Pro).", true,
		enumArg("mode", "Mode", "auto", false, choice("auto", "Auto"), choice("none", "None"), choice("always", "Always")),
		floatArg("weight", "Weight", "1", 0, 100, false),
		floatArg("divider", "Divider", "24", 0, 1000, false)),

	// ---- Overlays ---------------------------------------------------------
	opt("background", "Background", "Overlays", "Fill transparent areas with a color.", false,
		colorArg("color", "Color", "ffffff")),
	opt("background_alpha", "Background alpha", "Overlays", "Alpha for the background color (imgproxy Pro).", true,
		floatArg("alpha", "Alpha", "1", 0, 1, false)),
	opt("watermark", "Watermark", "Overlays", "Place the configured watermark image.", false,
		floatArg("opacity", "Opacity", "1", 0, 1, false),
		enumArg("position", "Position", "ce", false,
			choice("ce", "Center"), choice("no", "North"), choice("so", "South"), choice("ea", "East"), choice("we", "West"),
			choice("noea", "North east"), choice("nowe", "North west"), choice("soea", "South east"), choice("sowe", "South west"),
			choice("re", "Repeat")),
		stringArg("x_offset", "X offset", "", "0.5 or 100"),
		stringArg("y_offset", "Y offset", "", "0.5 or 100"),
		floatArg("scale", "Scale", "0", 0, 10, false),
	),
	opt("watermark_url", "Watermark URL", "Overlays", "Use a URL as the watermark (imgproxy Pro).", true,
		base64Arg("url", "URL", true)),
	opt("watermark_text", "Watermark text", "Overlays", "Render text as the watermark (imgproxy Pro).", true,
		base64Arg("text", "Text", true)),
	opt("watermark_size", "Watermark size", "Overlays", "Target watermark size (imgproxy Pro).", true,
		intArg("width", "Width", "0", 0, 4096, false),
		intArg("height", "Height", "0", 0, 4096, false)),
	opt("watermark_rotate", "Watermark rotate", "Overlays", "Rotate the watermark (imgproxy Pro).", true,
		intArg("angle", "Angle", "0", -360, 360, false)),
	opt("watermark_shadow", "Watermark shadow", "Overlays", "Add a shadow to the watermark (imgproxy Pro).", true,
		floatArg("sigma", "Sigma", "1", 0, 100, false)),
	opt("colorize", "Colorize", "Overlays", "Place a color overlay (imgproxy Pro).", true,
		floatArg("opacity", "Opacity", "1", 0, 1, false),
		colorArg("color", "Color", "000000"),
		boolArg("keep_alpha", "Keep alpha", "0")),
	opt("gradient", "Gradient", "Overlays", "Place a gradient overlay (imgproxy Pro).", true),

	// ---- Metadata ---------------------------------------------------------
	opt("strip_metadata", "Strip metadata", "Metadata", "Remove EXIF/IPTC metadata.", false,
		boolArg("strip_metadata", "Strip metadata", "1")),
	opt("keep_copyright", "Keep copyright", "Metadata", "Keep copyright when stripping metadata.", false,
		boolArg("keep_copyright", "Keep copyright", "1")),
	opt("strip_color_profile", "Strip color profile", "Metadata", "Convert to sRGB and remove the ICC profile.", false,
		boolArg("strip_color_profile", "Strip color profile", "1")),
	opt("preserve_hdr", "Preserve HDR", "Metadata", "Keep high-bit images high-bit.", false,
		boolArg("preserve_hdr", "Preserve HDR", "1")),
	opt("enforce_thumbnail", "Enforce thumbnail", "Metadata", "Use the embedded thumbnail when present.", false,
		boolArg("enforce_thumbnail", "Enforce thumbnail", "1")),
	opt("dpi", "DPI", "Metadata", "Replace the DPI metadata (imgproxy Pro).", true,
		intArg("dpi", "DPI", "0", 0, 10000, false)),
	opt("color_profile", "Color profile", "Metadata", "Convert to a named ICC profile (imgproxy Pro).", true,
		stringArg("profile", "Profile", "srgb", "srgb, cmyk or a filename")),

	// ---- Output -----------------------------------------------------------
	opt("skip_processing", "Skip processing", "Output", "Return the source unchanged for these formats.", false),
	opt("raw", "Raw", "Output", "Return the source without processing.", false,
		boolArg("raw", "Raw", "1")),
	opt("cachebuster", "Cache buster", "Output", "Change to bypass caches without changing the image.", false,
		stringArg("value", "Value", "", "build-42")),
	opt("expires", "Expires", "Output", "Reject the request after this Unix timestamp.", false,
		intArg("timestamp", "Unix timestamp", "0", 0, 1<<40, false)),
	opt("filename", "Filename", "Output", "Set the Content-Disposition filename.", false,
		base64Arg("filename", "Filename", true)),
	opt("return_attachment", "Return attachment", "Output", "Use attachment in Content-Disposition.", false,
		boolArg("return_attachment", "Return attachment", "1")),
	opt("fallback_image_url", "Fallback image URL", "Output", "Fallback image when the source cannot be loaded (imgproxy Pro).", true,
		base64Arg("url", "URL", true)),

	// ---- Pages & animation ------------------------------------------------
	opt("page", "Page", "Pages & animation", "Page/frame to use (imgproxy Pro).", true,
		intArg("page", "Page", "0", 0, 10000, false)),
	opt("pages", "Pages", "Pages & animation", "Number of pages to stack (imgproxy Pro).", true,
		intArg("pages", "Pages", "1", 1, 1000, false)),
	opt("disable_animation", "Disable animation", "Pages & animation", "Treat animated images as still images (imgproxy Pro).", true,
		boolArg("disable_animation", "Disable animation", "1")),

	// ---- Video (Pro) ------------------------------------------------------
	opt("video_thumbnail_second", "Video thumbnail second", "Video (Pro)", "Timestamp for video thumbnails.", true,
		floatArg("second", "Second", "1", 0, 1e6, false)),
	opt("video_thumbnail_keyframes", "Video thumbnail keyframes", "Video (Pro)", "Prefer keyframes for video thumbnails.", true,
		boolArg("keyframes", "Keyframes", "1")),
	opt("video_thumbnail_tile", "Video thumbnail tile", "Video (Pro)", "Generate a tiled video sprite.", true),
	opt("video_thumbnail_animation", "Video thumbnail animation", "Video (Pro)", "Generate an animated video preview.", true),

	// ---- Advanced (Pro) ---------------------------------------------------
	opt("blur_areas", "Blur areas", "Advanced (Pro)", "Blur specified areas.", true),
	opt("blur_detections", "Blur detections", "Advanced (Pro)", "Blur detected objects.", true),
	opt("draw_detections", "Draw detections", "Advanced (Pro)", "Draw detected object boxes.", true),
	opt("crop_objects", "Crop objects", "Advanced (Pro)", "Crop around detected objects.", true),
	opt("style", "SVG style", "Advanced (Pro)", "Inject CSS into an SVG source.", true,
		base64Arg("style", "CSS", true)),
	opt("jpeg_options", "JPEG options", "Advanced (Pro)", "Advanced JPEG encoder options.", true),
	opt("png_options", "PNG options", "Advanced (Pro)", "Advanced PNG encoder options.", true),
	opt("webp_options", "WebP options", "Advanced (Pro)", "Advanced WebP encoder options.", true),
	opt("avif_options", "AVIF options", "Advanced (Pro)", "Advanced AVIF encoder options.", true),
	opt("hashsum", "Hashsum", "Advanced (Pro)", "Verify the source hash before processing.", true),
}

var catalogByName = func() map[string]CatalogOption {
	out := make(map[string]CatalogOption, len(catalogOptions))
	for _, option := range catalogOptions {
		out[option.Name] = option
	}
	return out
}()

// imgproxy accepts short aliases for most processing options. Presets are
// stored with canonical names, but signed ad-hoc requests may use the normal
// imgproxy aliases.
var catalogAliases = map[string]string{
	"rs": "resize", "s": "size", "rt": "resizing_type", "ra": "resizing_algorithm",
	"w": "width", "h": "height", "mw": "min_width", "mh": "min_height", "z": "zoom",
	"el": "enlarge", "ex": "extend", "exar": "extend_aspect_ratio", "g": "gravity",
	"c": "crop", "car": "crop_aspect_ratio", "obj_pos": "objects_position", "op": "objects_position",
	"t": "trim", "pd": "padding",
	"ar": "auto_rotate", "rot": "rotate", "fl": "flip", "bg": "background", "bga": "background_alpha",
	"bl": "blur",
	"sh": "sharpen", "pix": "pixelate", "sm": "strip_metadata", "kcr": "keep_copyright",
	"scp": "strip_color_profile", "ph": "preserve_hdr", "eth": "enforce_thumbnail",
	"q": "quality", "fq": "format_quality", "mb": "max_bytes", "aq": "autoquality",
	"a": "adjust", "br": "brightness", "co": "contrast", "sa": "saturation", "mc": "monochrome",
	"dt": "duotone", "ush": "unsharp_masking", "wm": "watermark", "wmu": "watermark_url",
	"wmt": "watermark_text", "wms": "watermark_size", "wmr": "watermark_rotate",
	"wmsh": "watermark_shadow", "col": "colorize", "gr": "gradient", "cp": "color_profile",
	"icc": "color_profile", "skp": "skip_processing", "cb": "cachebuster", "exp": "expires",
	"fn": "filename", "att": "return_attachment", "fiu": "fallback_image_url",
	"pg": "page", "pgs": "pages",
	"da": "disable_animation", "vts": "video_thumbnail_second", "vtk": "video_thumbnail_keyframes",
	"vtt": "video_thumbnail_tile", "vta": "video_thumbnail_animation", "ba": "blur_areas",
	"bd": "blur_detections", "dd": "draw_detections", "c_obj": "crop_objects", "st": "style",
	"jpgo": "jpeg_options", "pngo": "png_options", "webpo": "webp_options",
	"avifo": "avif_options", "hs": "hashsum",
}

func canonicalOptionName(name string) string {
	if canonical, ok := catalogAliases[name]; ok {
		return canonical
	}
	return name
}

var imageFormats = []string{"jpg", "png", "webp", "gif", "avif"}

var (
	presetNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	colorPattern      = regexp.MustCompile(`^[0-9a-fA-F]{3,8}$`)
	base64Pattern     = regexp.MustCompile(`^[A-Za-z0-9_=-]+$`)
)

func ImgProxyCatalog() Catalog {
	options := make([]CatalogOption, len(catalogOptions))
	copy(options, catalogOptions)
	return Catalog{Formats: append([]string(nil), imageFormats...), Options: options}
}

func LookupCatalogOption(name string) (CatalogOption, bool) {
	option, ok := catalogByName[name]
	return option, ok
}

func NormalizeFormat(format string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "jpeg" {
		format = "jpg"
	}
	for _, allowed := range imageFormats {
		if format == allowed {
			return format, nil
		}
	}
	return "", fmt.Errorf("%w: unsupported output format %q", ErrInvalidInput, format)
}

// ValidatePreset normalizes and validates a preset's label, name and parameters.
func ValidatePreset(label, name string, params domain.PresetParameters) (string, string, domain.PresetParameters, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return "", "", params, fmt.Errorf("%w: label is required", ErrInvalidInput)
	}
	if len(label) > 100 {
		return "", "", params, fmt.Errorf("%w: label is too long", ErrInvalidInput)
	}

	name = strings.ToLower(strings.TrimSpace(name))
	if !presetNamePattern.MatchString(name) {
		return "", "", params, fmt.Errorf("%w: name must be a lowercase slug (a-z, 0-9, dash, underscore)", ErrInvalidInput)
	}
	if isSizeName(name) {
		return "", "", params, fmt.Errorf("%w: %q is reserved for size conversions", ErrInvalidInput, name)
	}

	normalized, err := ValidatePresetParameters(params)
	if err != nil {
		return "", "", params, err
	}
	return label, name, normalized, nil
}

func ValidatePresetParameters(params domain.PresetParameters) (domain.PresetParameters, error) {
	format, err := NormalizeFormat(params.Format)
	if err != nil {
		return params, err
	}
	options, err := NormalizePresetOptions(params.Options)
	if err != nil {
		return params, err
	}
	return domain.PresetParameters{Format: format, Options: options}, nil
}

func NormalizePresetOptions(options []domain.PresetOption) ([]domain.PresetOption, error) {
	if len(options) == 0 {
		return nil, fmt.Errorf("%w: add at least one manipulation", ErrInvalidInput)
	}
	if len(options) > 32 {
		return nil, fmt.Errorf("%w: too many manipulations (max 32)", ErrInvalidInput)
	}

	seen := make(map[string]bool, len(options))
	normalized := make([]domain.PresetOption, 0, len(options))
	for _, input := range options {
		name := canonicalOptionName(strings.TrimSpace(input.Name))
		if seen[name] {
			return nil, fmt.Errorf("%w: %s is used more than once", ErrInvalidInput, name)
		}
		seen[name] = true

		definition, ok := LookupCatalogOption(name)
		if !ok {
			return nil, fmt.Errorf("%w: unknown imgproxy option %q", ErrInvalidInput, name)
		}
		option, err := normalizeOption(definition, input)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, option)
	}
	return normalized, nil
}

func BuildProcessing(params domain.PresetParameters) (string, error) {
	normalized, err := ValidatePresetParameters(params)
	if err != nil {
		return "", err
	}
	parts := make([]string, 0, len(normalized.Options))
	for _, option := range normalized.Options {
		parts = append(parts, strings.Join(append([]string{option.Name}, option.Args...), ":"))
	}
	return strings.Join(parts, "/"), nil
}

// ParseProcessing validates a normal imgproxy processing string (for example
// "rs:fit:800:600/q:82") and returns the normalized option list.
func ParseProcessing(processing string) ([]domain.PresetOption, error) {
	parts := strings.Split(strings.Trim(processing, "/"), "/")
	options := make([]domain.PresetOption, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		fields := strings.Split(part, ":")
		options = append(options, domain.PresetOption{
			Name: fields[0],
			Args: fields[1:],
		})
	}
	return NormalizePresetOptions(options)
}

func normalizeOption(definition CatalogOption, input domain.PresetOption) (domain.PresetOption, error) {
	if definition.Raw {
		args := trimTrailingEmpty(input.Args)
		if len(args) == 0 {
			return domain.PresetOption{}, fmt.Errorf("%w: %s requires arguments", ErrInvalidInput, definition.Name)
		}
		if len(args) > 16 {
			return domain.PresetOption{}, fmt.Errorf("%w: too many arguments for %s", ErrInvalidInput, definition.Name)
		}
		for _, value := range args {
			if err := validateSafeArg(value); err != nil {
				return domain.PresetOption{}, fmt.Errorf("%w: %s: %v", ErrInvalidInput, definition.Name, err)
			}
		}
		return domain.PresetOption{Name: definition.Name, Args: args}, nil
	}

	args := make([]string, 0, len(definition.Args))
	for i, spec := range definition.Args {
		value := ""
		if i < len(input.Args) {
			value = strings.TrimSpace(input.Args[i])
		}
		if value == "" {
			value = spec.Default
		}
		if value == "" {
			if spec.Required {
				return domain.PresetOption{}, fmt.Errorf("%w: %s.%s is required", ErrInvalidInput, definition.Name, spec.Name)
			}
			args = append(args, "")
			continue
		}
		normalized, err := validateArg(definition.Name, spec, value)
		if err != nil {
			return domain.PresetOption{}, err
		}
		args = append(args, normalized)
	}
	args = trimTrailingEmpty(args)
	if len(args) == 0 {
		return domain.PresetOption{}, fmt.Errorf("%w: %s requires arguments", ErrInvalidInput, definition.Name)
	}
	return domain.PresetOption{Name: definition.Name, Args: args}, nil
}

func validateArg(option string, spec CatalogArg, value string) (string, error) {
	switch spec.Kind {
	case ArgInt:
		number, err := strconv.Atoi(value)
		if err != nil {
			return "", fmt.Errorf("%w: %s.%s must be an integer", ErrInvalidInput, option, spec.Name)
		}
		if err := checkRange(option, spec, float64(number)); err != nil {
			return "", err
		}
		return strconv.Itoa(number), nil
	case ArgFloat:
		number, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return "", fmt.Errorf("%w: %s.%s must be a number", ErrInvalidInput, option, spec.Name)
		}
		if err := checkRange(option, spec, number); err != nil {
			return "", err
		}
		return strconv.FormatFloat(number, 'f', -1, 64), nil
	case ArgBool:
		switch strings.ToLower(value) {
		case "1", "t", "true":
			return "1", nil
		case "0", "f", "false":
			return "0", nil
		default:
			return "", fmt.Errorf("%w: %s.%s must be true or false", ErrInvalidInput, option, spec.Name)
		}
	case ArgEnum:
		for _, choice := range spec.Options {
			if value == choice.Value {
				return value, nil
			}
		}
		return "", fmt.Errorf("%w: %s.%s has an unsupported value", ErrInvalidInput, option, spec.Name)
	case ArgColor:
		if !colorPattern.MatchString(value) {
			return "", fmt.Errorf("%w: %s.%s must be a hex color", ErrInvalidInput, option, spec.Name)
		}
		return strings.ToLower(value), nil
	case ArgBase64:
		if !base64Pattern.MatchString(value) {
			return "", fmt.Errorf("%w: %s.%s must be URL-safe base64", ErrInvalidInput, option, spec.Name)
		}
		return value, nil
	default:
		if err := validateSafeArg(value); err != nil {
			return "", fmt.Errorf("%w: %s.%s: %v", ErrInvalidInput, option, spec.Name, err)
		}
		return value, nil
	}
}

func checkRange(option string, spec CatalogArg, value float64) error {
	if spec.Min != nil && value < *spec.Min {
		return fmt.Errorf("%w: %s.%s is below %v", ErrInvalidInput, option, spec.Name, *spec.Min)
	}
	if spec.Max != nil && value > *spec.Max {
		return fmt.Errorf("%w: %s.%s is above %v", ErrInvalidInput, option, spec.Name, *spec.Max)
	}
	return nil
}

func validateSafeArg(value string) error {
	if strings.ContainsAny(value, "/:?#@ \t\r\n") {
		return fmt.Errorf("contains an unsupported character")
	}
	if len(value) > 512 {
		return fmt.Errorf("is too long")
	}
	return nil
}

func trimTrailingEmpty(values []string) []string {
	last := len(values) - 1
	for last >= 0 && strings.TrimSpace(values[last]) == "" {
		last--
	}
	return values[:last+1]
}

func opt(name, label, category, description string, pro bool, args ...CatalogArg) CatalogOption {
	return CatalogOption{
		Name:        name,
		Label:       label,
		Category:    category,
		Description: description,
		Pro:         pro,
		Raw:         len(args) == 0,
		Args:        args,
	}
}

func intArg(name, label, def string, min, max float64, required bool) CatalogArg {
	return CatalogArg{Name: name, Label: label, Kind: ArgInt, Default: def, Min: floatPtr(min), Max: floatPtr(max), Required: required}
}

func floatArg(name, label, def string, min, max float64, required bool) CatalogArg {
	return CatalogArg{Name: name, Label: label, Kind: ArgFloat, Default: def, Min: floatPtr(min), Max: floatPtr(max), Required: required}
}

func boolArg(name, label, def string) CatalogArg {
	return CatalogArg{Name: name, Label: label, Kind: ArgBool, Default: def}
}

func enumArg(name, label, def string, required bool, choices ...CatalogChoice) CatalogArg {
	return CatalogArg{Name: name, Label: label, Kind: ArgEnum, Default: def, Required: required, Options: choices}
}

func stringArg(name, label, def, placeholder string) CatalogArg {
	return CatalogArg{Name: name, Label: label, Kind: ArgString, Default: def, Placeholder: placeholder}
}

func colorArg(name, label, def string) CatalogArg {
	return CatalogArg{Name: name, Label: label, Kind: ArgColor, Default: def, Placeholder: "RRGGBB"}
}

func base64Arg(name, label string, required bool) CatalogArg {
	return CatalogArg{Name: name, Label: label, Kind: ArgBase64, Required: required}
}

func choice(value, label string) CatalogChoice {
	return CatalogChoice{Value: value, Label: label}
}

func gravityChoices() []CatalogChoice {
	return []CatalogChoice{
		choice("ce", "Center"), choice("no", "North"), choice("so", "South"),
		choice("ea", "East"), choice("we", "West"), choice("noea", "North east"),
		choice("nowe", "North west"), choice("soea", "South east"), choice("sowe", "South west"),
		choice("sm", "Smart"), choice("fp", "Focus point"),
	}
}

func floatPtr(value float64) *float64 {
	return &value
}
