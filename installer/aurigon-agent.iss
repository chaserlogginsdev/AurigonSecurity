; ─────────────────────────────────────────────────────────────────────────────
; Aurigon Security Agent — Installer
;
; Requirements:
;   - Inno Setup 6.x  (https://jrsoftware.org/isinfo.php)
;   - Build the agent before running this script:
;       cd agent && go build -o ..\aurigon-agent.exe .\cmd\agent
;   - Place nssm.exe in installer\nssm.exe
;
; To compile:
;   "C:\Program Files (x86)\Inno Setup 6\ISCC.exe" installer\aurigon-agent.iss
;
; Silent install:
;   AurigonAgentSetup.exe /SILENT /AGENTKEY="AGT-..."
; ─────────────────────────────────────────────────────────────────────────────

#define AppName     "Aurigon Security Agent"
#define AppVersion  "1.0.0"
#define AppPublisher "Aurigon Security"
#define AppURL      "https://aurigonsecurity.com"
#define ServiceName "AurigonAgent"

[Setup]
AppId={{A4E2C1B3-7F8D-4A9E-B012-3C5D6E7F8A9B}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisherURL={#AppURL}
DefaultDirName={commonpf64}\AurigonSecurity\Agent
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes
OutputDir=.
OutputBaseFilename=AurigonAgentSetup
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
UninstallDisplayName={#AppName}
UninstallDisplayIcon={app}\aurigon-agent.exe
VersionInfoVersion={#AppVersion}
VersionInfoCompany={#AppPublisher}
VersionInfoDescription={#AppName} Setup

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Files]
Source: "..\aurigon-agent.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "nssm.exe";             DestDir: "{app}"; Flags: ignoreversion

[UninstallRun]
Filename: "{app}\nssm.exe"; Parameters: "stop {#ServiceName}";           Flags: runhidden waituntilterminated; RunOnceId: "StopSvc"
Filename: "{app}\nssm.exe"; Parameters: "remove {#ServiceName} confirm"; Flags: runhidden waituntilterminated; RunOnceId: "RemoveSvc"

; ─────────────────────────────────────────────────────────────────────────────
; One field only — the permanent tenant agent key
; ─────────────────────────────────────────────────────────────────────────────
[Code]

var
  AgentKeyPage: TInputQueryWizardPage;

procedure InitializeWizard();
begin
  AgentKeyPage := CreateInputQueryPage(
    wpWelcome,
    'Agent Configuration',
    'Enter your Aurigon agent key',
    'Find this on your dashboard under Download Agent. It is the same ' +
    'key for every machine in your workspace — no per-machine setup required.'
  );

  AgentKeyPage.Add('Agent Key:', False);

  AgentKeyPage.Values[0] := ExpandConstant('{param:AGENTKEY|}');
end;

function NextButtonClick(CurPageID: Integer): Boolean;
var
  AgentKey: String;
begin
  Result := True;

  if CurPageID = AgentKeyPage.ID then begin
    AgentKey := Trim(AgentKeyPage.Values[0]);

    if AgentKey = '' then begin
      MsgBox('Please enter your agent key.', mbError, MB_OK);
      Result := False;
      Exit;
    end;

    if Copy(AgentKey, 1, 4) <> 'AGT-' then begin
      MsgBox('Invalid agent key — it should start with "AGT-". Please check the key and try again.', mbError, MB_OK);
      Result := False;
      Exit;
    end;
  end;
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  AgentKey: String;
  ResultCode: Integer;
begin
  if CurStep = ssPostInstall then begin
    AgentKey := Trim(AgentKeyPage.Values[0]);
    if AgentKey = '' then
      AgentKey := ExpandConstant('{param:AGENTKEY|}');

    Exec(ExpandConstant('{app}\nssm.exe'),
      'install {#ServiceName} "' + ExpandConstant('{app}\aurigon-agent.exe') + '"',
      '', SW_HIDE, ewWaitUntilTerminated, ResultCode);

    Exec(ExpandConstant('{app}\nssm.exe'),
      'set {#ServiceName} AppDirectory "' + ExpandConstant('{app}') + '"',
      '', SW_HIDE, ewWaitUntilTerminated, ResultCode);

    Exec(ExpandConstant('{app}\nssm.exe'),
      'set {#ServiceName} AppRestartDelay 5000',
      '', SW_HIDE, ewWaitUntilTerminated, ResultCode);

    Exec(ExpandConstant('{app}\nssm.exe'),
      'set {#ServiceName} AppEnvironmentExtra "AURIGON_AGENT_KEY=' + AgentKey + '"',
      '', SW_HIDE, ewWaitUntilTerminated, ResultCode);

    Exec(ExpandConstant('{app}\nssm.exe'),
      'start {#ServiceName}',
      '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  end;
end;

function UpdateReadyMemo(Space, NewLine, MemoUserInfoInfo, MemoDirInfo,
  MemoTypeInfo, MemoComponentsInfo, MemoGroupInfo, MemoTasksInfo: String): String;
begin
  Result := MemoDirInfo + NewLine + NewLine +
    'The Aurigon agent will be installed as a Windows service and ' +
    'will start automatically on every boot.' + NewLine + NewLine +
    'Your machine will appear in the Aurigon dashboard within 30 seconds.';
end;
