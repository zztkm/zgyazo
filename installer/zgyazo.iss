; Inno Setup Script for zgyazo
; Based on Ollama's installer implementation

#define MyAppName "zgyazo"
#define MyAppVersion "1.0.0"
#define MyAppPublisher "zztkm"
#define MyAppURL "https://github.com/zztkm/zgyazo"
#define MyAppExeName "zgyazo.exe"

[Setup]
; NOTE: AppId uniquely identifies this application in Windows
AppId={{B87D65A7-5F27-4723-B4B3-9C7D8E7F6A53}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
DefaultDirName={localappdata}\Programs\{#MyAppName}
DefaultGroupName={#MyAppName}
AllowNoIcons=yes
LicenseFile=..\LICENSE
OutputDir=..\dist
OutputBaseFilename=zgyazoSetup
Compression=lzma
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
MinVersion=10.0.10240
ArchitecturesAllowed=x64
ArchitecturesInstallIn64BitMode=x64
UninstallDisplayIcon={app}\{#MyAppExeName}
ShowLanguageDialog=no
ChangesEnvironment=yes

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "japanese"; MessagesFile: "compiler:Languages\Japanese.isl"

[CustomMessages]
InstallingApp=Installing {#MyAppName}...
RunningApp=Starting {#MyAppName}...
UninstallingApp=Uninstalling {#MyAppName}...
english.InstallingApp=Installing {#MyAppName}...
english.RunningApp=Starting {#MyAppName}...
english.UninstallingApp=Uninstalling {#MyAppName}...
japanese.InstallingApp={#MyAppName} をインストールしています...
japanese.RunningApp={#MyAppName} を起動しています...
japanese.UninstallingApp={#MyAppName} をアンインストールしています...

[Tasks]
Name: "autostart"; Description: "Start {#MyAppName} when Windows starts"; GroupDescription: "Additional options:"; Flags: unchecked

[Files]
Source: "..\dist\zgyazo.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.md"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{group}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"
Name: "{userstartup}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: autostart

[Registry]
; Add zgyazo to PATH
Root: HKCU; Subkey: "Environment"; ValueType: expandsz; ValueName: "Path"; ValueData: "{olddata};{app}"; Check: NeedsAddPath('{app}')

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent

[UninstallRun]
; Stop zgyazo before uninstalling
Filename: "taskkill"; Parameters: "/F /IM {#MyAppExeName}"; Flags: runhidden; RunOnceId: "StopZgyazo"

[UninstallDelete]
Type: filesandordirs; Name: "{localappdata}\zgyazo"

[Code]
function NeedsAddPath(Param: string): boolean;
var
  OrigPath: string;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER,
    'Environment',
    'Path', OrigPath)
  then begin
    Result := True;
    exit;
  end;
  // Look for the path with leading and trailing semicolon
  // Pos() returns 0 if not found
  Result := Pos(';' + Param + ';', ';' + OrigPath + ';') = 0;
end;

procedure RemovePath(Path: string);
var
  Paths: string;
  P: Integer;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', Paths) then
    exit;
  
  P := Pos(';' + Path + ';', ';' + Paths + ';');
  if P = 0 then
    exit;
  
  Delete(Paths, P - 1, Length(Path) + 1);
  RegWriteStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', Paths);
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usPostUninstall then
    RemovePath(ExpandConstant('{app}'));
end;

// Check if zgyazo is running and offer to close it
function PrepareToInstall(var NeedsRestart: Boolean): String;
var
  ResultCode: Integer;
begin
  if Exec('tasklist', '/FI "IMAGENAME eq {#MyAppExeName}" | find "{#MyAppExeName}"', '', SW_HIDE, ewWaitUntilTerminated, ResultCode) then
  begin
    if ResultCode = 0 then
    begin
      if MsgBox('{#MyAppName} is currently running. Setup must close it to continue. Do you want Setup to close {#MyAppName} now?', mbConfirmation, MB_YESNO) = IDYES then
      begin
        Exec('taskkill', '/F /IM {#MyAppExeName}', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
        Sleep(1000); // Give it a moment to close
      end
      else
      begin
        Result := '{#MyAppName} is running. Please close it and run Setup again.';
      end;
    end;
  end;
end;