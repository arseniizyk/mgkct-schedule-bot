// Telegram Mini App Client for MGKCT Schedule Bot
(() => {
  const tg = window.Telegram?.WebApp;
  if (tg) {
    tg.ready();
    tg.expand();
  }

  // State
  const state = {
    mode: 'group', // 'group' | 'teacher' | 'calls'
    groups: [],
    teachers: [],
    facultyDirectory: [],
    subjects: {},
    calls: [],
    selectedGroup: null,
    selectedTeacher: '',
    selectedWeek: '',
    weeksNav: null,
    schedule: null, // Group or Teacher entity
    currentDayIdx: 0,
    userProfile: null,
  };

  // DOM Elements
  const tabGroup = document.getElementById('tabGroup');
  const tabTeacher = document.getElementById('tabTeacher');
  const tabCalls = document.getElementById('tabCalls');
  const themeToggle = document.getElementById('themeToggle');
  const scheduleToolbar = document.getElementById('scheduleToolbar');
  const scheduleView = document.getElementById('scheduleView');
  const callsView = document.getElementById('callsView');
  const targetLabel = document.getElementById('targetLabel');
  const targetSelect = document.getElementById('targetSelect');
  const prevWeekBtn = document.getElementById('prevWeekBtn');
  const nextWeekBtn = document.getElementById('nextWeekBtn');
  const weekLabel = document.getElementById('weekLabel');
  const dayTabs = document.getElementById('dayTabs');
  const currentDayTitle = document.getElementById('currentDayTitle');
  const pairsCountLabel = document.getElementById('pairsCountLabel');
  const saveDefaultBtn = document.getElementById('saveDefaultBtn');
  const pairsList = document.getElementById('pairsList');
  const callsList = document.getElementById('callsList');
  const loadingIndicator = document.getElementById('loadingIndicator');
  const errorState = document.getElementById('errorState');
  const errorMessage = document.getElementById('errorMessage');
  const retryBtn = document.getElementById('retryBtn');

  const dayNames = ['Понедельник', 'Вторник', 'Среда', 'Четверг', 'Пятница', 'Суббота'];

  // Haptic feedback helper
  function haptic(type = 'light') {
    if (tg?.HapticFeedback) {
      if (type === 'success' || type === 'error' || type === 'warning') {
        tg.HapticFeedback.notificationOccurred(type);
      } else {
        tg.HapticFeedback.impactOccurred(type);
      }
    }
  }

  // Fetch with Telegram Authorization
  async function apiFetch(url, options = {}) {
    const headers = options.headers || {};
    if (tg?.initData) {
      headers['Authorization'] = `tma ${tg.initData}`;
    }
    const res = await fetch(url, { ...options, headers });
    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || `HTTP ${res.status}`);
    }
    return res.json();
  }

  // Theme Sync & Management
  const darkThemeVars = {
    '--tg-theme-bg-color': '#18181b',
    '--tg-theme-secondary-bg-color': '#27272a',
    '--tg-theme-text-color': '#fafafa',
    '--tg-theme-hint-color': '#a1a1aa',
    '--tg-theme-link-color': '#3b82f6',
    '--tg-theme-button-color': '#2563eb',
    '--tg-theme-button-text-color': '#ffffff',
    '--tg-theme-header-bg-color': '#18181b',
    '--tg-theme-section-bg-color': '#27272a',
    '--tg-theme-section-separator-color': '#3f3f46',
  };

  const lightThemeVars = {
    '--tg-theme-bg-color': '#ffffff',
    '--tg-theme-secondary-bg-color': '#f4f4f5',
    '--tg-theme-text-color': '#000000',
    '--tg-theme-hint-color': '#71717a',
    '--tg-theme-link-color': '#2481cc',
    '--tg-theme-button-color': '#2481cc',
    '--tg-theme-button-text-color': '#ffffff',
    '--tg-theme-header-bg-color': '#ffffff',
    '--tg-theme-section-bg-color': '#ffffff',
    '--tg-theme-section-separator-color': '#e4e4e7',
  };

  function applyTheme(isDark) {
    const vars = isDark ? darkThemeVars : lightThemeVars;
    if (isDark) {
      document.documentElement.classList.add('dark');
    } else {
      document.documentElement.classList.remove('dark');
    }
    for (const [key, val] of Object.entries(vars)) {
      document.documentElement.style.setProperty(key, val);
    }
  }

  function syncTheme() {
    const saved = localStorage.getItem('mgkct_theme');
    if (saved === 'dark') {
      applyTheme(true);
      return;
    }
    if (saved === 'light') {
      applyTheme(false);
      return;
    }
    const isDark = tg?.colorScheme === 'dark' || window.matchMedia('(prefers-color-scheme: dark)').matches;
    applyTheme(isDark);
  }

  if (tg) {
    tg.onEvent('themeChanged', () => {
      if (!localStorage.getItem('mgkct_theme')) {
        syncTheme();
      }
    });
  }

  themeToggle.addEventListener('click', () => {
    const isDark = document.documentElement.classList.contains('dark');
    const newDark = !isDark;
    localStorage.setItem('mgkct_theme', newDark ? 'dark' : 'light');
    applyTheme(newDark);
    haptic('light');
  });

  // Calculate day index based on current local day & time
  function getInitialDayIndex() {
    const now = new Date();
    const day = now.getDay(); // 0 is Sunday, 1 is Monday ... 6 is Saturday
    if (day === 0) return 0; // Sunday -> show Monday
    const idx = day - 1; // 0..5
    // If it's Saturday after 18:00, show Monday
    if (idx === 5 && now.getHours() >= 18) {
      return 0;
    }
    return Math.min(idx, 5);
  }

  // Check if given pair number is current right now
  function isPairNow(pairNum) {
    if (!state.calls || state.calls.length === 0) return false;
    const call = state.calls.find(c => c.number === pairNum);
    if (!call) return false;

    const now = new Date();
    const [startH, startM] = call.start.split(':').map(Number);
    const [endH, endM] = call.end.split(':').map(Number);

    const startTime = new Date();
    startTime.setHours(startH, startM, 0, 0);

    const endTime = new Date();
    endTime.setHours(endH, endM, 0, 0);

    return now >= startTime && now <= endTime;
  }

  // Format Date for Week button
  function formatDateShort(dateStr) {
    if (!dateStr) return '';
    const d = new Date(dateStr);
    if (isNaN(d.getTime())) return '';
    return d.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit' });
  }

  // Switch App Mode
  function setMode(mode) {
    haptic('light');
    state.mode = mode;

    [tabGroup, tabTeacher, tabCalls].forEach(tab => {
      tab.className = 'py-2 rounded-xl text-[var(--tg-theme-hint-color)] hover:text-[var(--tg-theme-text-color)] transition text-center flex items-center justify-center';
    });

    if (mode === 'group') {
      tabGroup.className = 'py-2 rounded-xl bg-[var(--tg-theme-bg-color)] shadow-xs text-[var(--tg-theme-button-color)] font-semibold transition text-center flex items-center justify-center';
      scheduleToolbar.classList.remove('hidden');
      scheduleView.classList.remove('hidden');
      callsView.classList.add('hidden');
      targetLabel.textContent = 'Группа:';
      saveDefaultBtn.textContent = 'Моя группа';
      renderTargetOptions();
      loadSchedule();
    } else if (mode === 'teacher') {
      tabTeacher.className = 'py-2 rounded-xl bg-[var(--tg-theme-bg-color)] shadow-xs text-[var(--tg-theme-button-color)] font-semibold transition text-center flex items-center justify-center';
      scheduleToolbar.classList.remove('hidden');
      scheduleView.classList.remove('hidden');
      callsView.classList.add('hidden');
      targetLabel.textContent = 'Преподаватель:';
      saveDefaultBtn.textContent = 'Моё расписание';
      renderTargetOptions();
      loadSchedule();
    } else if (mode === 'calls') {
      tabCalls.className = 'py-2 rounded-xl bg-[var(--tg-theme-bg-color)] shadow-xs text-[var(--tg-theme-button-color)] font-semibold transition text-center flex items-center justify-center';
      scheduleToolbar.classList.add('hidden');
      scheduleView.classList.add('hidden');
      callsView.classList.remove('hidden');
      renderCalls();
    }
  }

  tabGroup.addEventListener('click', () => setMode('group'));
  tabTeacher.addEventListener('click', () => setMode('teacher'));
  tabCalls.addEventListener('click', () => setMode('calls'));

  // Render Target Select Dropdown
  function renderTargetOptions() {
    targetSelect.innerHTML = '';
    if (state.mode === 'group') {
      state.groups.forEach(g => {
        const opt = document.createElement('option');
        opt.value = g;
        opt.textContent = `Группа ${g}`;
        if (g == state.selectedGroup) opt.selected = true;
        targetSelect.appendChild(opt);
      });
    } else if (state.mode === 'teacher') {
      const facMap = {};
      (state.facultyDirectory || []).forEach(f => {
        if (f.short_name) facMap[f.short_name] = f;
      });

      state.teachers.forEach(t => {
        const opt = document.createElement('option');
        opt.value = t;
        const fac = facMap[t];
        if (fac && fac.full_name && fac.full_name !== t) {
          opt.textContent = `${t} (${fac.full_name})`;
        } else {
          opt.textContent = t;
        }
        if (t === state.selectedTeacher) opt.selected = true;
        targetSelect.appendChild(opt);
      });
    }
  }

  targetSelect.addEventListener('change', e => {
    haptic('light');
    state.selectedWeek = ''; // Reset to latest
    if (state.mode === 'group') {
      state.selectedGroup = parseInt(e.target.value, 10);
    } else {
      state.selectedTeacher = e.target.value;
    }
    loadSchedule();
  });

  // Week Navigation
  prevWeekBtn.addEventListener('click', () => {
    if (state.weeksNav?.prev && state.weeksNav.prev !== '0001-01-01T00:00:00Z') {
      haptic('light');
      state.selectedWeek = state.weeksNav.prev.split('T')[0];
      loadSchedule();
    }
  });

  nextWeekBtn.addEventListener('click', () => {
    if (state.weeksNav?.next && state.weeksNav.next !== '0001-01-01T00:00:00Z') {
      haptic('light');
      state.selectedWeek = state.weeksNav.next.split('T')[0];
      loadSchedule();
    }
  });

  // Save Default Button
  saveDefaultBtn.addEventListener('click', async () => {
    haptic('light');
    try {
      const payload = {};
      if (state.mode === 'group') {
        payload.group_id = state.selectedGroup;
        localStorage.setItem('mgkct_saved_mode', 'group');
        localStorage.setItem('mgkct_saved_group', String(state.selectedGroup));
      } else {
        payload.teacher_name = state.selectedTeacher;
        localStorage.setItem('mgkct_saved_mode', 'teacher');
        localStorage.setItem('mgkct_saved_teacher', state.selectedTeacher);
      }

      await apiFetch('/api/v1/user/profile', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      haptic('success');
      if (tg?.showAlert) {
        tg.showAlert('Сохранено в ваш профиль бота!');
      } else {
        alert('Сохранено в ваш профиль бота!');
      }
    } catch (err) {
      haptic('error');
      console.error('Save profile error:', err);
      alert('Ошибка при сохранении профиля');
    }
  });

  // Load Schedule Data
  async function loadSchedule() {
    loadingIndicator.classList.remove('hidden');
    scheduleView.classList.add('hidden');
    errorState.classList.add('hidden');

    try {
      let url = '';
      if (state.mode === 'group') {
        if (!state.selectedGroup) {
          loadingIndicator.classList.add('hidden');
          return;
        }
        url = `/api/v1/schedule/group?id=${state.selectedGroup}`;
        if (state.selectedWeek) url += `&week=${state.selectedWeek}`;
      } else {
        if (!state.selectedTeacher) {
          loadingIndicator.classList.add('hidden');
          return;
        }
        url = `/api/v1/schedule/teacher?name=${encodeURIComponent(state.selectedTeacher)}`;
        if (state.selectedWeek) url += `&week=${state.selectedWeek}`;
      }

      const res = await apiFetch(url);
      state.schedule = state.mode === 'group' ? res.group : res.teacher;
      state.weeksNav = res.weeks;

      updateWeekBar();
      renderDaysTabs();
      renderDaySchedule();

      loadingIndicator.classList.add('hidden');
      scheduleView.classList.remove('hidden');
    } catch (err) {
      console.error('Failed to load schedule:', err);
      loadingIndicator.classList.add('hidden');
      errorMessage.textContent = 'Расписание не найдено или ещё не загружено';
      errorState.classList.remove('hidden');
    }
  }

  retryBtn.addEventListener('click', () => {
    haptic('light');
    loadSchedule();
  });

  // Update Week Navigation Label & Buttons
  function updateWeekBar() {
    const nav = state.weeksNav;
    if (!nav) return;

    const hasPrev = nav.prev && nav.prev !== '0001-01-01T00:00:00Z';
    const hasNext = nav.next && nav.next !== '0001-01-01T00:00:00Z';

    prevWeekBtn.disabled = !hasPrev;
    nextWeekBtn.disabled = !hasNext;

    if (nav.current && nav.current !== '0001-01-01T00:00:00Z') {
      const cur = new Date(nav.current);
      const end = new Date(cur);
      end.setDate(cur.getDate() + 5);
      weekLabel.textContent = `${formatDateShort(cur.toISOString())} - ${formatDateShort(end.toISOString())}`;
    } else {
      weekLabel.textContent = 'Неделя';
    }
  }

  // Check if the given day object matches today's real date
  function isDayToday(day) {
    if (!day) return false;

    // 1. Try parsing exact date from day.name (e.g. "Понедельник | 12.10.2026")
    if (day.name) {
      const match = day.name.match(/(\d{2})\.(\d{2})\.(\d{4})/);
      if (match) {
        const now = new Date();
        const d = parseInt(match[1], 10);
        const m = parseInt(match[2], 10) - 1;
        const y = parseInt(match[3], 10);
        return now.getDate() === d && now.getMonth() === m && now.getFullYear() === y;
      }
    }

    // 2. Fallback: compare day index and week date
    const now = new Date();
    const dayOfWeek = (now.getDay() + 6) % 7; // Monday = 0 ... Sunday = 6
    if (dayOfWeek > 5) return false;
    if (state.currentDayIdx !== dayOfWeek) return false;

    if (state.schedule?.week) {
      const curMonday = new Date(now.getFullYear(), now.getMonth(), now.getDate() - dayOfWeek);
      curMonday.setHours(0, 0, 0, 0);
      const schedWeek = new Date(state.schedule.week);
      schedWeek.setHours(0, 0, 0, 0);
      return Math.abs(curMonday.getTime() - schedWeek.getTime()) <= 48 * 3600 * 1000;
    }

    return true;
  }

  const shortDayNames = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб'];

  // Render Day Selector Tabs
  function renderDaysTabs() {
    dayTabs.innerHTML = '';
    const days = state.schedule?.days || [];

    for (let i = 0; i < 6; i++) {
      const btn = document.createElement('button');
      btn.dataset.day = i;
      const isSelected = i === state.currentDayIdx;

      btn.className = isSelected
        ? 'day-tab px-3 py-1.5 rounded-lg text-xs font-semibold whitespace-nowrap bg-[var(--tg-theme-button-color)] text-[var(--tg-theme-button-text-color)] shadow-xs'
        : 'day-tab px-3 py-1.5 rounded-lg text-xs font-medium whitespace-nowrap bg-[var(--tg-theme-secondary-bg-color)] text-[var(--tg-theme-text-color)] hover:opacity-80 transition';

      let label = dayNames[i];
      if (days[i]?.name) {
        const match = days[i].name.match(/(\d{2})\.(\d{2})/);
        if (match) {
          label = `${shortDayNames[i]}, ${match[1]}.${match[2]}`;
        }
      }
      btn.textContent = label;

      btn.addEventListener('click', () => {
        haptic('light');
        state.currentDayIdx = i;
        renderDaysTabs();
        renderDaySchedule();
      });

      dayTabs.appendChild(btn);
    }
  }

  function resolveSubjectName(rawName) {
    if (!rawName) return '';
    const cleaned = rawName.replace(/^[123]\.\s*/, '').replace(/^\d+\*?\s*[-–]\s*/, '').trim();
    return state.subjects?.[cleaned] || '';
  }

  // Render Selected Day Schedule
  function renderDaySchedule() {
    const days = state.schedule?.days || [];
    const day = days[state.currentDayIdx];

    currentDayTitle.firstElementChild.textContent = dayNames[state.currentDayIdx];

    if (!day || !day.subjects || day.subjects.length === 0) {
      pairsCountLabel.textContent = '0 пар запланировано';
      pairsList.innerHTML = `
        <div class="py-12 text-center text-xs text-[var(--tg-theme-hint-color)]">
          Занятий нет. Свободный день! 🎉
        </div>
      `;
      return;
    }

    // Собираем непустые слоты пар
    const validSlots = [];
    day.subjects.forEach((subject, idx) => {
      if (!subject.isEmpty && subject.pairs && subject.pairs.length > 0) {
        validSlots.push({ subject, pairNum: idx + 1 });
      }
    });

    pairsCountLabel.textContent = `${validSlots.length} пар запланировано`;

    if (validSlots.length === 0) {
      pairsList.innerHTML = `
        <div class="py-12 text-center text-xs text-[var(--tg-theme-hint-color)]">
          Занятий нет. Свободный день! 🎉
        </div>
      `;
      return;
    }

    const isToday = isDayToday(day);
    pairsList.innerHTML = '';
    const totalCount = validSlots.length;

    validSlots.forEach(({ subject, pairNum }, slotIdx) => {
      const call = state.calls.find(c => c.number === pairNum);
      const timeStr = call ? call.time : '';
      const isCurrent = isToday && isPairNow(pairNum);

      // Скругления Pixel Connected Stack
      let radiusClass = 'rounded-[6px]';
      if (totalCount === 1) {
        radiusClass = 'rounded-2xl';
      } else if (slotIdx === 0) {
        radiusClass = 'rounded-t-2xl rounded-b-[6px]';
      } else if (slotIdx === totalCount - 1) {
        radiusClass = 'rounded-b-2xl rounded-t-[6px]';
      }

      const activeClass = isCurrent
        ? 'border-2 border-[var(--tg-theme-button-color)] ring-2 ring-[var(--tg-theme-button-color)]/25'
        : 'border border-[var(--tg-theme-section-separator-color)] hover:border-[var(--tg-theme-button-color)]/30';

      const card = document.createElement('div');
      card.className = `p-2.5 px-3.5 bg-[var(--tg-theme-section-bg-color)] ${radiusClass} ${activeClass} transition shadow-2xs`;

      const isSingle = subject.pairs.length === 1;
      const singlePair = isSingle ? subject.pairs[0] : null;

      if (isSingle) {
        const teacherOrGroup = state.mode === 'group'
          ? (singlePair.group || '')
          : (singlePair.group ? `Группа ${singlePair.group}` : '');

        const cleanRoom = (singlePair.class && singlePair.class !== '-') ? singlePair.class : '';
        const roomBadgeHtml = cleanRoom
          ? `<span class="text-[11px] font-semibold px-2 py-0.5 rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 shrink-0">${escapeHtml(cleanRoom)}</span>`
          : '';

        const typeStr = singlePair.type ? ` (${escapeHtml(singlePair.type)})` : '';
        const fullSubj = resolveSubjectName(singlePair.name);
        const fullSubjHtml = fullSubj
          ? `<p class="text-[11px] text-[var(--tg-theme-hint-color)] mt-0.5 truncate">${escapeHtml(fullSubj)}</p>`
          : '';

        card.innerHTML = `
          <!-- Верхняя строка: Номер + Предмет (слева) и Кабинет (справа) -->
          <div class="flex items-center justify-between gap-2 mb-1">
            <div class="flex items-center gap-1.5 min-w-0">
              <span class="text-xs font-black ${isCurrent ? 'text-[var(--tg-theme-button-color)]' : 'text-[var(--tg-theme-hint-color)]'}">${pairNum}.</span>
              <span class="text-xs font-bold ${isCurrent ? 'text-[var(--tg-theme-button-color)]' : 'text-[var(--tg-theme-text-color)]'} truncate">${escapeHtml(singlePair.name)}</span>
            </div>
            ${roomBadgeHtml}
          </div>
          ${fullSubjHtml}
          <!-- Нижняя строка: Преподаватель (слева) и Время (справа) -->
          <div class="flex items-center justify-between text-[11px] text-[var(--tg-theme-hint-color)] mt-0.5">
            <span class="truncate pr-2">${escapeHtml(teacherOrGroup)}<span class="text-[10px] opacity-80">${typeStr}</span></span>
            <span class="text-[10px] font-medium shrink-0 tracking-tight">${timeStr}</span>
          </div>
        `;
      } else {
        // Подгруппы
        card.innerHTML = `
          <!-- Верхняя строка: Номер + Заголовок (слева) и Время (справа) -->
          <div class="flex items-center justify-between gap-2 mb-1.5">
            <div class="flex items-center gap-1.5 min-w-0">
              <span class="text-xs font-black ${isCurrent ? 'text-[var(--tg-theme-button-color)]' : 'text-[var(--tg-theme-hint-color)]'}">${pairNum}.</span>
              <span class="text-xs font-bold ${isCurrent ? 'text-[var(--tg-theme-button-color)]' : 'text-[var(--tg-theme-text-color)]'} truncate">Пара по подгруппам</span>
            </div>
            <span class="text-[10px] font-medium text-[var(--tg-theme-hint-color)] shrink-0 tracking-tight">${timeStr}</span>
          </div>

          <div class="space-y-1">
            ${subject.pairs.map((p, pIdx) => {
              const teacherOrGroup = state.mode === 'group'
                ? (p.group || '')
                : (p.group ? `Группа ${p.group}` : '');
              const cleanRoom = (p.class && p.class !== '-') ? p.class : '';
              const roomHtml = cleanRoom
                ? `<span class="font-bold text-[11px] text-emerald-600 dark:text-emerald-400 shrink-0 pl-2">${escapeHtml(cleanRoom)}</span>`
                : '';
              const typeStr = p.type ? ` (${escapeHtml(p.type)})` : '';
              const fullSubj = resolveSubjectName(p.name);
              const fullSubjTitle = fullSubj ? ` title="${escapeHtml(fullSubj)}"` : '';

              return `
                <div class="flex items-center justify-between text-[11px] p-1 px-2 rounded-lg bg-[var(--tg-theme-secondary-bg-color)] border border-[var(--tg-theme-section-separator-color)]/40">
                  <div class="flex items-center gap-1.5 min-w-0">
                    <span class="font-extrabold text-[10px] text-[var(--tg-theme-button-color)] bg-blue-500/10 px-1 rounded shrink-0">${pIdx + 1} п/г</span>
                    <span class="font-bold text-xs truncate" ${fullSubjTitle}>${escapeHtml(p.name)}</span>
                    <span class="text-[10px] text-[var(--tg-theme-hint-color)] truncate block">· ${escapeHtml(teacherOrGroup)}${typeStr}</span>
                  </div>
                  ${roomHtml}
                </div>
              `;
            }).join('')}
          </div>
        `;
      }

      pairsList.appendChild(card);
    });
  }

  // Render Calls View
  function renderCalls() {
    callsList.innerHTML = '';
    const now = new Date();
    const isSunday = now.getDay() === 0;

    state.calls.forEach(call => {
      const isCurrent = !isSunday && isPairNow(call.number);
      const row = document.createElement('div');
      row.className = `p-3.5 flex items-center justify-between ${isCurrent ? 'bg-[var(--tg-theme-button-color)]/5' : ''}`;

      row.innerHTML = `
        <div class="flex items-center gap-3">
          <span class="w-7 h-7 rounded-lg ${isCurrent ? 'bg-[var(--tg-theme-button-color)] text-[var(--tg-theme-button-text-color)]' : 'bg-blue-500/10 text-[var(--tg-theme-button-color)]'} font-bold text-xs flex items-center justify-center">${call.number}</span>
          <div>
            <div class="text-sm font-semibold flex items-center gap-2">
              <span>${call.number} пара</span>
              ${isCurrent ? '<span class="text-[10px] font-bold px-2 py-0.5 rounded-md bg-[var(--tg-theme-button-color)] text-[var(--tg-theme-button-text-color)]">ИДЕТ СЕЙЧАС</span>' : ''}
            </div>
            <div class="text-xs text-[var(--tg-theme-hint-color)]">${call.first_half} | ${call.second_half}</div>
          </div>
        </div>
        <span class="text-xs text-[var(--tg-theme-hint-color)] font-mono">${escapeHtml(call.break_duration)}</span>
      `;

      callsList.appendChild(row);
    });
  }

  function escapeHtml(str) {
    if (!str) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }

  // Initialize App Data
  async function init() {
    syncTheme();
    state.currentDayIdx = getInitialDayIndex();

    // 0. Восстановление сохраненного выбора из localStorage
    const savedMode = localStorage.getItem('mgkct_saved_mode');
    const savedGroup = localStorage.getItem('mgkct_saved_group');
    const savedTeacher = localStorage.getItem('mgkct_saved_teacher');
    if (savedMode === 'teacher' && savedTeacher) {
      state.mode = 'teacher';
      state.selectedTeacher = savedTeacher;
    } else if (savedMode === 'group' && savedGroup) {
      state.mode = 'group';
      state.selectedGroup = parseInt(savedGroup, 10);
    }

    try {
      // 1. Fetch Calls
      const callsRes = await apiFetch('/api/v1/calls');
      state.calls = callsRes.calls || [];

      // 2. Fetch Groups, Teachers & Subjects
      const [groupsRes, teachersRes, subjectsRes] = await Promise.all([
        apiFetch('/api/v1/groups'),
        apiFetch('/api/v1/teachers'),
        apiFetch('/api/v1/subjects'),
      ]);

      state.groups = groupsRes.groups || [];
      state.teachers = teachersRes.teachers || [];
      state.facultyDirectory = teachersRes.directory || [];
      state.subjects = subjectsRes.subjects || {};

      // 3. Fetch User Profile с сервера (синхронизация с ботом)
      try {
        const profile = await apiFetch('/api/v1/user/profile');
        state.userProfile = profile;
        if (profile.teacher_name) {
          state.mode = 'teacher';
          state.selectedTeacher = profile.teacher_name;
          localStorage.setItem('mgkct_saved_mode', 'teacher');
          localStorage.setItem('mgkct_saved_teacher', profile.teacher_name);
        } else if (profile.group_id && profile.group_id > 0) {
          state.mode = 'group';
          state.selectedGroup = profile.group_id;
          localStorage.setItem('mgkct_saved_mode', 'group');
          localStorage.setItem('mgkct_saved_group', String(profile.group_id));
        }
      } catch (e) {
        console.warn('User profile not authorized or empty:', e);
      }

      // Default selection if not set by profile or localStorage
      if (!state.selectedGroup && state.groups.length > 0) {
        state.selectedGroup = state.groups[0];
      }
      if (!state.selectedTeacher && state.teachers.length > 0) {
        state.selectedTeacher = state.teachers[0];
      }

      // Переключаем на нужную вкладку и загружаем расписание
      setMode(state.mode);
    } catch (err) {
      console.error('Initialization error:', err);
      loadingIndicator.classList.add('hidden');
      errorMessage.textContent = 'Не удалось загрузить данные с сервера';
      errorState.classList.remove('hidden');
    }
  }

  init();
})();
