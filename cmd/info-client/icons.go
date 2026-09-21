package main

import weatherpb "homeserver/gen/weather"

// icon returns a small multi-line ASCII glyph for the given weather category,
// as classified server-side by cmd/info-server (see its categorizeCode).
func icon(cat weatherpb.Category) []string {
	switch cat {
	case weatherpb.Category_CATEGORY_SUNNY:
		return []string{
			`  \   /  `,
			`   .-.   `,
			`- (   ) -`,
			`   ` + "`" + `-'   `,
			`  /   \  `,
		}
	case weatherpb.Category_CATEGORY_PARTLY_CLOUDY:
		return []string{
			`   \  /   `,
			` _ /""".-.`,
			`   \_(   )`,
			`   /(___(_`,
			`          `,
		}
	case weatherpb.Category_CATEGORY_CLOUDY:
		return []string{
			`          `,
			`    .--.  `,
			` .-(    ).`,
			`(___.__)__`,
			`          `,
		}
	case weatherpb.Category_CATEGORY_FOG:
		return []string{
			`          `,
			` _ - _ -  `,
			`  _ - _ - `,
			` _ - _ -  `,
			`          `,
		}
	case weatherpb.Category_CATEGORY_RAIN:
		return []string{
			`    .--.  `,
			` .-(    ).`,
			`(___.__)__`,
			` ' ' ' '  `,
			`          `,
		}
	case weatherpb.Category_CATEGORY_THUNDER:
		return []string{
			`    .--.  `,
			` .-(    ).`,
			`(___.__)__`,
			`  ! ' !  `,
			`          `,
		}
	case weatherpb.Category_CATEGORY_SNOW:
		return []string{
			`    .--.  `,
			` .-(    ).`,
			`(___.__)__`,
			`  *  *  * `,
			`          `,
		}
	case weatherpb.Category_CATEGORY_SLEET:
		return []string{
			`    .--.  `,
			` .-(    ).`,
			`(___.__)__`,
			`  * ' * ' `,
			`          `,
		}
	default:
		return []string{
			`          `,
			`    ?     `,
			`   ???    `,
			`    ?     `,
			`          `,
		}
	}
}
