return {
	"nvim/nvim-lspconfig",
	opts = {
		servers = {
			gopls = {
				settings = {
					gopls = {
						buildFlags = { "-tags=goolm" },
					},
				},
			},
		},
	},
}
