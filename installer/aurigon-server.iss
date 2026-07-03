; ─────────────────────────────────────────────────────────────────────────────
; Aurigon Security — Server Installer
; Installs the backend + dashboard as a Windows service
;
; Requirements:
;   - Inno Setup 6.x  (https://jrsoftware.org/isinfo.php)
;   - Build the backend and dashboard BEFORE running this script:
;
;       cd dashboard && npm run build
;       cd backend && go build -o aurigon-backend.exe .
;
;   - Place nssm.exe in installer\nssm.exe (copy from nssm-2.24\win64\)
;
; To compile:
;   "C:\Program Files (x86)\Inno Setup 6\ISCC.exe" installer\aurigon-server.iss
;
; Silent install:
;   AurigonServerSetup.exe /SILENT /AGENTKEY="mykey" /JWTSECRET="32charmin..." /ADMINPASS="mypassword"
; ─────────────────────────────────────────────────────────────────────────────

#define AppName      "Aurigon Security Server"
#define AppVersion   "1.0.0"
#define AppPublisher "Aurigon Security"
#define AppURL       "https://aurigonsecurity.com"
#define ServiceName  "AurigonBackend"
#define InstallDir   "{commonpf64}\AurigonSecurity"

[Setup]
AppId={{B7F3D2A1-9C4E-4B8F-A023-1D6E7F8A9B0C}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisherURL={#AppURL}
DefaultDirName={#InstallDir}
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes
OutputDir=.
OutputBaseFilename=AurigonServerSetup
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
UninstallDisplayName={#AppName}
UninstallDisplayIcon={app}\aurigon-backend.exe
VersionInfoVersion={#AppVersion}
VersionInfoCompany={#AppPublisher}
VersionInfoDescription={#AppName} Setup

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Files]
; Backend binary
Source: "..\backend\aurigon-backend.exe"; DestDir: "{app}"; Flags: ignoreversion

; Compiled dashboard (entire dist folder)
Source: "..\backend\dist\*"; DestDir: "{app}\dist"; Flags: ignoreversion recursesubdirs createallsubdirs

; NSSM — bundled so no dependency on the customer having it
Source: "nssm.exe"; DestDir: "{app}"; Flags: ignoreversion

[Run]
; Install and start the service after files are copied
Filename: "{app}\nssm.exe"; Parameters: "install {#ServiceName} ""{app}\aurigon-backend.exe"""; \
  Flags: runhidden waituntilterminated; \
  StatusMsg: "Installing Aurigon backend service..."

Filename: "{app}\nssm.exe"; Parameters: "set {#ServiceName} AppDirectory ""{app}"""; \
  Flags: runhidden waituntilterminated

Filename: "{app}\nssm.exe"; Parameters: "set {#ServiceName} AppRestartDelay 5000"; \
  Flags: runhidden waituntilterminated

Filename: "{app}\nssm.exe"; Parameters: "start {#ServiceName}"; \
  Flags: runhidden waituntilterminated; \
  StatusMsg: "Starting Aurigon backend service..."

[UninstallRun]
Filename: "{app}\nssm.exe"; Parameters: "stop {#ServiceName}";      Flags: runhidden waituntilterminated; RunOnceId: "StopSvc"
Filename: "{app}\nssm.exe"; Parameters: "remove {#ServiceName} confirm"; Flags: runhidden waituntilterminated; RunOnceId: "RemoveSvc"

; ─────────────────────────────────────────────────────────────────────────────
; Custom wizard pages — collect config before install
; ─────────────────────────────────────────────────────────────────────────────
[Code]

var
  ConfigPage: TInputQueryWizardPage;

procedure InitializeWizard();
begin
  ConfigPage := CreateInputQueryPage(
    wpWelcome,
    'Service Configuration',
    'Configure your Aurigon Security backend',
    'These settings will be stored securely and used every time the service starts.'
  );

  ConfigPage.Add('Agent Key (shared with all agents):', True);
  ConfigPage.Add('JWT Secret (min 32 characters):', True);
  ConfigPage.Add('Admin Password (for dashboard login):', True);

  // Pre-fill from silent install params if provided
  ConfigPage.Values[0] := ExpandConstant('{param:AGENTKEY|}');
  ConfigPage.Values[1] := ExpandConstant('{param:JWTSECRET|}');
  ConfigPage.Values[2] := ExpandConstant('{param:ADMINPASS|}');
end;

function NextButtonClick(CurPageID: Integer): Boolean;
var
  AgentKey, JWTSecret, AdminPass: String;
begin
  Result := True;

  if CurPageID = ConfigPage.ID then begin
    AgentKey  := Trim(ConfigPage.Values[0]);
    JWTSecret := Trim(ConfigPage.Values[1]);
    AdminPass := Trim(ConfigPage.Values[2]);

    if AgentKey = '' then begin
      MsgBox('Please enter an Agent Key.', mbError, MB_OK);
      Result := False;
      Exit;
    end;

    if Length(JWTSecret) < 32 then begin
      MsgBox('JWT Secret must be at least 32 characters.', mbError, MB_OK);
      Result := False;
      Exit;
    end;

    if AdminPass = '' then begin
      MsgBox('Please enter an Admin Password.', mbError, MB_OK);
      Result := False;
      Exit;
    end;
  end;
end;

// Write env vars to NSSM after files are installed but before service starts
procedure CurStepChanged(CurStep: TSetupStep);
var
  AgentKey, JWTSecret, AdminPass: String;
  ResultCode: Integer;
begin
  if CurStep = ssPostInstall then begin
    AgentKey  := Trim(ConfigPage.Values[0]);
    JWTSecret := Trim(ConfigPage.Values[1]);
    AdminPass := Trim(ConfigPage.Values[2]);

    // Fall back to silent install params if wizard values are empty
    if AgentKey  = '' then AgentKey  := ExpandConstant('{param:AGENTKEY|}');
    if JWTSecret = '' then JWTSecret := ExpandConstant('{param:JWTSECRET|}');
    if AdminPass = '' then AdminPass := ExpandConstant('{param:ADMINPASS|}');

    // Set environment variables on the service via NSSM
    Exec(
      ExpandConstant('{app}\nssm.exe'),
      'set ' + '{#ServiceName}' + ' AppEnvironmentExtra ' +
        '"AURIGON_AGENT_KEY=' + AgentKey + '" ' +
        '"AURIGON_JWT_SECRET=' + JWTSecret + '" ' +
        '"AURIGON_ADMIN_PASSWORD=' + AdminPass + '"',
      '',
      SW_HIDE,
      ewWaitUntilTerminated,
      ResultCode
    );
  end;
end;

// Show finish message with dashboard URL
function UpdateReadyMemo(Space, NewLine, MemoUserInfoInfo, MemoDirInfo,
  MemoTypeInfo, MemoComponentsInfo, MemoGroupInfo, MemoTasksInfo: String): String;
begin
  Result := MemoDirInfo + NewLine + NewLine +
    'After installation, open your browser and go to:' + NewLine +
    Space + 'http://localhost:8080' + NewLine + NewLine +
    'Log in with username: admin' + NewLine +
    'Password: the Admin Password you entered above.';
end;

// Clean up registry on uninstall
procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usPostUninstall then begin
    // Data directory is kept — don't delete the database on uninstall
  end;
end;
