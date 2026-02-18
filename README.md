# Go Minesweeper (Windows-style)

고전 윈도우 지뢰찾기 느낌을 최대한 살려 만든 Go + Ebiten 버전입니다.

## 포함 기능

- ✅ Beginner / Intermediate / Expert 난이도 (`1`,`2`,`3` 또는 `B`,`I`,`E`)
- ✅ Custom 보드 설정 다이얼로그 (`C`)
- ✅ 좌클릭 오픈, 우클릭 마킹(깃발/물음표 순환)
- ✅ 숫자 셀 chord(주변 깃발 수 일치 시 주변 오픈)
- ✅ 첫 클릭 안전 + 주변 8칸 보호
- ✅ 지뢰 카운터 / 타이머(디지털 표시)
- ✅ 스마일 버튼(즉시 재시작)
- ✅ 힌트 기능 (`H`) - 안전한 칸 하이라이트
- ✅ 일시정지 (`P`)
- ✅ 테마 전환 (`T`) - Classic / Dark
- ✅ 최고 기록 저장 + 보기 (`S`)
- ✅ 도움말 오버레이 (`F1`)

## 실행

```bash
go run .
```

## 빌드

```bash
go build -o minesweeper.exe .
```

## 웹(폰 브라우저 포함) 실행

### 1) WebAssembly 빌드

```powershell
./build-web.ps1
```

### 2) 정적 서버 실행

```bash
python -m http.server 8080 -d web
```

### 3) 접속
- PC: `http://localhost:8080`
- 폰: `http://<PC의_로컬_IP>:8080`
  - PC/폰이 같은 Wi-Fi에 있어야 합니다.

> 외부 네트워크에서 접속하려면 HTTPS 호스팅(Vercel/Netlify/GitHub Pages 등) 권장

## 조작 키 / 터치

- `N`: 새 게임
- `1`/`2`/`3`: 초급/중급/고급
- `C`: 커스텀 보드 설정
- `H`: 힌트
- `P`: 일시정지
- `T`: 테마 변경
- `S`: 최고기록 보기
- `Q`: 물음표 마킹 사용 on/off
- `F1`: 도움말
- 터치(모바일/웹):
  - **탭**: 열기 / 코드(chord)
  - **길게 누르기(약 0.36초)**: 깃발/물음표 마킹

## 커스텀 설정 (C)

- `←/→`: 항목 선택 (Width/Height/Mines)
- `↑/↓`: 값 증감
- `Enter`: 적용 후 시작
- `Esc`: 취소

## 로컬 저장

최고 기록은 사용자 설정 폴더에 저장됩니다.
- Windows 예: `%AppData%\go-minesweeper\scores.json`

## 개발 메모

리소스 이미지는 외부 저작물 사용 없이, 코드로 직접 UI를 그리는 방식으로 구현했습니다.
(윈도우 클래식 지뢰찾기 감성 유지)
