; Windows installer of the app, built by scripts/windows-installer.sh.
;
; mygo makes one too, but with the default NSIS artwork and an English
; checkbox on the finish page. This script draws the app's own artwork
; (wizard.bmp, from assets/installer/wizard.svg) and speaks the three
; languages of the app. @NAME@, @VERSION@, @ID@ and @EXE@ are filled in by
; the shell script; app.exe, app.ico and wizard.bmp sit next to this file
; in the temporary directory makensis runs in.

Unicode true
ManifestDPIAware true
SetCompressor /SOLID lzma

Name "@NAME@"
OutFile "out.exe"
InstallDir "$LOCALAPPDATA\Programs\@NAME@"
InstallDirRegKey HKCU "Software\@ID@" "InstallDir"
RequestExecutionLevel user
BrandingText " "

!define MUI_ICON "app.ico"
!define MUI_UNICON "app.ico"
!define MUI_WELCOMEFINISHPAGE_BITMAP "wizard.bmp"
!define MUI_FINISHPAGE_RUN "$INSTDIR\@EXE@"
!define MUI_FINISHPAGE_SHOWREADME
!define MUI_FINISHPAGE_SHOWREADME_TEXT "$(DESKTOP_SHORTCUT)"
!define MUI_FINISHPAGE_SHOWREADME_FUNCTION CreateDesktopShortcut
!include "MUI2.nsh"

!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "SimpChinese"
!insertmacro MUI_LANGUAGE "Japanese"
!insertmacro MUI_LANGUAGE "English"

LangString DESKTOP_SHORTCUT ${LANG_SIMPCHINESE} "创建桌面快捷方式"
LangString DESKTOP_SHORTCUT ${LANG_JAPANESE} "デスクトップにショートカットを作成"
LangString DESKTOP_SHORTCUT ${LANG_ENGLISH} "Create a desktop shortcut"

!define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\@ID@"

Section "Install"
  SetOutPath "$INSTDIR"
  File "/oname=@EXE@" "app.exe"
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  CreateShortCut "$SMPROGRAMS\@NAME@.lnk" "$INSTDIR\@EXE@"
  WriteRegStr HKCU "Software\@ID@" "InstallDir" "$INSTDIR"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayName" "@NAME@"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayVersion" "@VERSION@"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\@EXE@"
  WriteRegStr HKCU "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINST_KEY}" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegStr HKCU "${UNINST_KEY}" "QuietUninstallString" '"$INSTDIR\Uninstall.exe" /S'
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoRepair" 1
SectionEnd

; Only the finish page offers it, so silent installs (/S) make none.
Function CreateDesktopShortcut
  SetOutPath "$INSTDIR"
  CreateShortCut "$DESKTOP\@NAME@.lnk" "$INSTDIR\@EXE@"
FunctionEnd

Section "Uninstall"
  Delete "$SMPROGRAMS\@NAME@.lnk"
  Delete "$DESKTOP\@NAME@.lnk"
  DeleteRegKey HKCU "${UNINST_KEY}"
  DeleteRegKey HKCU "Software\@ID@"
  ; The folder goes as a whole: an install made by an older installer may
  ; hold files this one does not know about.
  RMDir /r "$INSTDIR"
SectionEnd
