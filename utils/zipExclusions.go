package utils

var ExcludedFiles []string = []string{
	// ignore projectCode.zip
	"projectCode.zip",
	"**/projectCode.zip",
	"./projectCode.zip",
	// ignore all node_modules files
	"**/node_modules/*",
	"./node_modules/*",
	"node_modules/*",
	"**/node_modules",
	"./node_modules",
	"node_modules",
	"node_modules/**",
	"**/node_modules/**",
	// ignore all .git files
	"**/.git/*",
	"./.git/*",
	".git/*",
	"**/.git",
	"./.git",
	".git",
	".git/**",
	"**/.git/**",
	// ignore all .next files
	"**/.next/*",
	"./.next/*",
	".next/*",
	"**/.next",
	"./.next",
	".next",
	".next/**",
	"**/.next/**",
	// ignore all .open-next files
	"**/.open-next/*",
	"./.open-next/*",
	".open-next/*",
	"**/.open-next",
	"./.open-next",
	".open-next",
	".open-next/**",
	"**/.open-next/**",
	// ignore all .vercel files
	"**/.vercel/*",
	"./.vercel/*",
	".vercel/*",
	"**/.vercel",
	"./.vercel",
	".vercel",
	".vercel/**",
	"**/.vercel/**",
	// ignore all .turbo files
	"**/.turbo/*",
	"./.turbo/*",
	".turbo/*",
	"**/.turbo",
	"./.turbo",
	".turbo",
	".turbo/**",
	"**/.turbo/**",
	// ignore all .sst files
	"**/.sst/*",
	"./.sst/*",
	".sst/*",
	"**/.sst",
	"./.sst",
	".sst",
	".sst/**",
	"**/.sst/**",
}
