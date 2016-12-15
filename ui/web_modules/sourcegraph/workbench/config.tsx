import { IModelService } from "vs/editor/common/services/modelService";
import { IModeService } from "vs/editor/common/services/modeService";
import { ITextModelResolverService } from "vs/editor/common/services/resolverService";
import { IConfigurationValue, getConfigurationValue } from "vs/platform/configuration/common/configuration";
import { ServiceCollection } from "vs/platform/instantiation/common/serviceCollection";

import { code_font_face } from "sourcegraph/components/styles/_vars.css";
import { TextModelContentProvider } from "sourcegraph/editor/resolverService";
import { Features } from "sourcegraph/util/features";

// Workbench overwrites a few services, so we add these services after startup.
export function configurePostStartup(services: ServiceCollection): void {
	const resolver = services.get(ITextModelResolverService) as ITextModelResolverService;
	resolver.registerTextModelContentProvider("git", new TextModelContentProvider(
		services.get(IModelService) as IModelService,
		services.get(IModeService) as IModeService,
	));
}

export class ConfigurationService {
	readonly config: Object = {
		workbench: {
			quickOpen: {
				closeOnFocusLost: false,
			},
			editor: {
				enablePreview: false,
			},
		},
		explorer: {
			openEditors: {
				visible: 0,
			},
		},
		editor: {
			readOnly: true,
			automaticLayout: true,
			scrollBeyondLastLine: false,
			wrappingColumn: 0,
			fontFamily: code_font_face,
			fontSize: 15,
			lineHeight: 21,
			theme: "vs-dark",
			renderLineHighlight: "line",
			codeLens: Features.codeLens.isEnabled(),
			glyphMargin: false,
		},
	};

	getConfiguration(key: string): any {
		return this.config;
	}


	lookup(key: string): IConfigurationValue<any> {
		return {
			value: getConfigurationValue(this.config, key),
			default: getConfigurationValue(this.config, key),
			user: getConfigurationValue(this.config, key),
		};
	}

	onDidUpdateConfiguration(): void {
		//
	}
}
