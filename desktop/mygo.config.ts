// The Desktop App as MyGo packages it. This file is evaluated by Node or Bun
// and imports nothing, so the module needs no JavaScript dependencies.
//
// `go build` and `go test` never read it. `go tool mygo generate`, `dev`, and
// `build` do; `go run ./cmd/devtool package` stages the resources below and
// then runs the build.
import { existsSync, readFileSync } from "node:fs";

// One release identity: the product version is what the app reports and what
// the updater compares. macOS wants a plain numeric version beside it, so a
// prerelease label never reaches CFBundleShortVersionString.
const version = process.env.LUMILIO_VERSION ?? "0.0.0";
const platformVersion = process.env.LUMILIO_PLATFORM_VERSION ?? version.split("-")[0];
const buildNumber = process.env.LUMILIO_BUILD_NUMBER ?? platformVersion;

// Signed updates are on only for a build that trusts a key. The public half
// is committed as build/update.pub once `go tool mygo keygen` has made a
// pair; the secret half lives in the release workflow's secrets and signs
// what `mygo build` publishes. Without the file the app is built without an
// update feed and says so in Settings.
const updateKeyFile = "build/update.pub";
const updates = existsSync(updateKeyFile)
  ? {
      publicKey: readFileSync(updateKeyFile, "utf8").trim(),
      github: "EdwinZhanCN/Lumilio-Photos",
    }
  : undefined;

export default {
  name: "Lumilio Photos",
  slug: "lumilio-photos",
  // The identifier the released bundles already carry: macOS keys folder
  // permissions and the login item on it.
  identifier: "com.edwinzhan.lumilio-photos",
  version,
  copyright: "© Lumilio Photos",
  publisher: "Lumilio Photos",
  license: "GPL-3.0-only",
  icon: "build/appicon.png",
  out: "bin",

  i18n: { dir: "internal/locales", source: "en" },

  // desktop/resources ships whole: the media tools `devtool fetch-resources`
  // puts there, and the Web SPA, licenses and native libraries that
  // `devtool stage` adds. What the Go binary embeds instead stays out.
  resourcesExclude: ["README.md", "manifest.json", "payload"],

  updates,

  macos: {
    icon: "build/appicon.icon",
    minimumSystemVersion: "12.0",
    infoPlist: {
      CFBundleShortVersionString: platformVersion,
      CFBundleVersion: buildNumber,
      // A menu-bar app: no Dock icon, even before the first window.
      LSUIElement: true,
      NSLocalNetworkUsageDescription:
        "Lumilio Photos discovers Lumen Intelligence servers on your local network via mDNS to enable optional AI features (semantic search, face recognition, OCR).",
      NSDesktopFolderUsageDescription:
        "Lumilio Photos needs access to folders you select on the Desktop to store and manage your photo repositories.",
      NSDocumentsFolderUsageDescription:
        "Lumilio Photos needs access to folders you select in Documents to store and manage your photo repositories.",
      NSDownloadsFolderUsageDescription:
        "Lumilio Photos needs access to folders you select in Downloads to import or manage your photo repositories.",
      NSRemovableVolumesUsageDescription:
        "Lumilio Photos needs ongoing access to external drives you select as Storage Locations for your photo repositories.",
      NSNetworkVolumesUsageDescription:
        "Lumilio Photos needs ongoing access to network folders you select as Storage Locations for your photo repositories.",
      NSFileProviderDomainUsageDescription:
        "Lumilio Photos needs access to folders you select in iCloud Drive or other file provider storage to manage your photo repositories.",
      NSBonjourServices: ["_lumen._tcp"],
    },
  },
};
