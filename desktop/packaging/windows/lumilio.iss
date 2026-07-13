; Inno Setup script for Lumilio Photos (Windows x64, per-user install).
;
; Packages the portable app directory produced by desktop/scripts/build-windows.sh
; into a single setup.exe that:
;   - installs per-user to %LocalAppData%\Programs\Lumilio Photos (no UAC),
;   - ensures the Microsoft Edge WebView2 Runtime (required by the first-run
;     onboarding window) is present, downloading Microsoft's Evergreen
;     bootstrapper and installing it silently if it is missing,
;   - creates Start Menu (and optional Desktop) shortcuts,
;   - registers an "Apps & features" entry with an uninstaller that stops the
;     running app + its bundled PostgreSQL and can optionally remove the app data.
;
; Storage-path selection is intentionally NOT done here: it belongs to the app's
; first-run onboarding window (per-user %LocalAppData%, with live writability
; validation), which only renders once WebView2 (ensured above) is installed.
;
; Compile (on Windows, with Inno Setup 6.1+):
;   ISCC.exe /DAppVersion=1.2.3 desktop\packaging\windows\lumilio.iss
; Override the payload location with /DPayloadDir=... if not the default below.

#ifndef AppVersion
  #define AppVersion "0.0.0"
#endif
#ifndef PayloadDir
  #define PayloadDir "..\..\build\windows\Lumilio Photos"
#endif

#define MyAppName "Lumilio Photos"
#define MyAppExeName "lumilio-photos.exe"
#define MyAppPublisher "EdwinZhan"
#define MyAppURL "https://github.com/EdwinZhanCN/Lumilio-Photos"

[Setup]
; A stable AppId keeps upgrades and the uninstall entry consistent across versions.
AppId={{7B9A2E4C-3F1D-4A6B-9C82-1E5F0A7D2C40}
AppName={#MyAppName}
AppVersion={#AppVersion}
AppVerName={#MyAppName} {#AppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
DefaultDirName={autopf}\{#MyAppName}
DisableProgramGroupPage=yes
; Per-user install: no administrator rights, no UAC prompt. {autopf} then resolves
; to %LocalAppData%\Programs (the VS Code / Discord model).
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir={#SourcePath}\..\..\build
OutputBaseFilename=Lumilio-Photos-{#AppVersion}-windows-amd64-setup
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
SetupIconFile={#SourcePath}\..\..\assets\lumilio-photos.ico
UninstallDisplayName={#MyAppName}
UninstallDisplayIcon={app}\lumilio-photos.ico
SetupLogging=yes

[Languages]
; English chrome only; the app itself is bilingual (zh/en) at first run. A
; ChineseSimplified.isl entry can be added later without changing the flow.
Name: "en"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

; On upgrade, wipe the previous program-files tree before copying the new
; payload. {app} is only binaries/resources (user data lives under
; %LocalAppData%\Lumilio Photos), so this is safe and prevents orphaned DLLs /
; tools from an older build from lingering next to the new ones.
[InstallDelete]
Type: filesandordirs; Name: "{app}\*"

[Files]
Source: "{#PayloadDir}\*"; DestDir: "{app}"; Flags: recursesubdirs createallsubdirs ignoreversion
Source: "{#SourcePath}\..\..\assets\lumilio-photos.ico"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\lumilio-photos.ico"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\lumilio-photos.ico"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#MyAppName}}"; Flags: nowait postinstall skipifsilent

[Code]
const
  { EdgeUpdate client GUID for the WebView2 Evergreen Runtime. }
  WV2_CLIENT = 'SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}';
  WV2_CLIENT_WOW = 'SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}';
  WV2_BOOTSTRAPPER_URL = 'https://go.microsoft.com/fwlink/p/?LinkId=2124703';

{ Stop a running Lumilio Photos instance and its child PostgreSQL, so program
  files are not locked during (re)install or uninstall. /T kills the process
  tree, taking the bundled postmaster with it; PostgreSQL is crash-safe (WAL) and
  the supervisor cleans up a stale postmaster.pid on next launch. }
procedure StopRunningApp();
var
  ResultCode: Integer;
begin
  Exec(ExpandConstant('{cmd}'), '/C taskkill /F /IM {#MyAppExeName} /T', '',
    SW_HIDE, ewWaitUntilTerminated, ResultCode);
end;

function WebView2Runtime(RootKey: Integer; SubKey: String): Boolean;
var
  pv: String;
begin
  Result := False;
  if RegQueryStringValue(RootKey, SubKey, 'pv', pv) then
    Result := (pv <> '') and (pv <> '0.0.0.0');
end;

function IsWebView2Installed(): Boolean;
begin
  Result := WebView2Runtime(HKCU, WV2_CLIENT)
         or WebView2Runtime(HKLM, WV2_CLIENT)
         or WebView2Runtime(HKLM, WV2_CLIENT_WOW);
end;

function EnsureWebView2(): String;
var
  BootstrapperPath: String;
  ResultCode: Integer;
begin
  Result := '';
  if IsWebView2Installed() then
    Exit;

  try
    DownloadTemporaryFile(WV2_BOOTSTRAPPER_URL, 'MicrosoftEdgeWebview2Setup.exe', '', nil);
  except
    if MsgBox('Lumilio Photos needs the Microsoft Edge WebView2 Runtime, which could not be downloaded'
      + ' (no internet connection?).' + #13#10#13#10
      + 'You can install it later from:' + #13#10
      + 'https://developer.microsoft.com/microsoft-edge/webview2/' + #13#10#13#10
      + 'Continue installing Lumilio Photos anyway?', mbConfirmation, MB_YESNO) = IDYES then
      Exit
    else begin
      Result := 'The Microsoft Edge WebView2 Runtime is required.';
      Exit;
    end;
  end;

  BootstrapperPath := ExpandConstant('{tmp}\MicrosoftEdgeWebview2Setup.exe');
  if not Exec(BootstrapperPath, '/silent /install', '', SW_HIDE, ewWaitUntilTerminated, ResultCode) then
    Result := 'Failed to launch the WebView2 Runtime installer.'
  else if ResultCode <> 0 then
    Result := Format('WebView2 Runtime installation failed (exit code %d).', [ResultCode]);
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
begin
  StopRunningApp();
  Result := EnsureWebView2();
end;

{ Read the user's chosen media-library location from desktop-settings.json so the
  uninstaller can warn when "remove data" would also delete the photo library
  (the default library lives inside the data directory). }
function GetStoragePath(DataDir: String): String;
var
  Content: AnsiString;
  S: String;
  P, Q: Integer;
begin
  Result := '';
  if not LoadStringFromFile(DataDir + '\config\desktop-settings.json', Content) then
    Exit;
  S := String(Content);
  P := Pos('"storage_path"', S);
  if P = 0 then
    Exit;
  S := Copy(S, P + Length('"storage_path"'), Length(S));
  P := Pos(':', S);
  if P = 0 then Exit;
  S := Copy(S, P + 1, Length(S));
  P := Pos('"', S);
  if P = 0 then Exit;
  S := Copy(S, P + 1, Length(S));
  Q := Pos('"', S);
  if Q = 0 then Exit;
  Result := Copy(S, 1, Q - 1);
  { JSON escapes backslashes; unescape \\ -> \ }
  StringChangeEx(Result, '\\', '\', True);
end;

function InitializeUninstall(): Boolean;
begin
  StopRunningApp();
  Sleep(1500);
  Result := True;
end;

procedure CurUninstallStepChanged(CurStep: TUninstallStep);
var
  DataDir, StoragePath: String;
  LibraryInsideData: Boolean;
begin
  if CurStep <> usPostUninstall then
    Exit;

  DataDir := ExpandConstant('{localappdata}\{#MyAppName}');
  if not DirExists(DataDir) then
    Exit;

  if MsgBox('Also remove all Lumilio Photos data (database, thumbnails, settings, logs) in:' + #13#10#13#10
    + DataDir + #13#10#13#10
    + 'Choose No to keep it for a future reinstall.', mbConfirmation, MB_YESNO or MB_DEFBUTTON2) <> IDYES then
    Exit;

  StoragePath := GetStoragePath(DataDir);
  { Default library ('' => <data>\storage) or any library nested under the data
    dir means "remove data" would delete the user's originals. Require an
    explicit second confirmation. An external library (e.g. D:\Photos) is never
    touched — it lives outside DataDir, which is all we delete. }
  LibraryInsideData := (StoragePath = '')
    or (Pos(Lowercase(DataDir), Lowercase(StoragePath)) = 1);

  if LibraryInsideData then
    if MsgBox('WARNING: your photo library is stored inside this folder.' + #13#10
      + 'Deleting it will PERMANENTLY delete your original photos and videos.' + #13#10#13#10
      + 'Delete everything, including your photos?', mbError, MB_YESNO or MB_DEFBUTTON2) <> IDYES then
      Exit;

  DelTree(DataDir, True, True, True);
end;
