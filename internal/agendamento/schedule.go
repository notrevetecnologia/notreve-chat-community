package agendamento

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	TipoOnce     = "once"
	TipoInterval = "interval"
	TipoCron     = "cron"

	AcaoEnviarMensagem  = "enviar_mensagem"
	AcaoRodarFerramenta = "rodar_ferramenta"
	AcaoCallbackAgente  = "callback_agente"

	StatusPendente   = "pendente"
	StatusExecutando = "executando"
	StatusConcluida  = "concluida"
	StatusFalhou     = "falhou"
	StatusCancelada  = "cancelada"

	minIntervalSeconds = 10
)

var (
	ErrTipoInvalido   = errors.New("tipo de agendamento inválido")
	ErrQuandoInvalido = errors.New("expressão de agendamento inválida")
)

func NormalizeTipo(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "once", "uma_vez", "unica":
		return TipoOnce
	case "interval", "intervalo":
		return TipoInterval
	case "cron":
		return TipoCron
	}
	return strings.ToLower(strings.TrimSpace(value))
}

func NormalizeAcao(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "enviar_mensagem", "mensagem":
		return AcaoEnviarMensagem
	case "rodar_ferramenta", "ferramenta":
		return AcaoRodarFerramenta
	case "callback_agente", "agente", "callback":
		return AcaoCallbackAgente
	}
	return strings.ToLower(strings.TrimSpace(value))
}

func ValidTipo(value string) bool {
	switch NormalizeTipo(value) {
	case TipoOnce, TipoInterval, TipoCron:
		return true
	}
	return false
}

func ValidAcao(value string) bool {
	switch NormalizeAcao(value) {
	case AcaoEnviarMensagem, AcaoRodarFerramenta, AcaoCallbackAgente:
		return true
	}
	return false
}

func brazilLocation() *time.Location {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return time.FixedZone("BRT", -3*60*60)
	}
	return loc
}

var onceLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
}

func parseOnce(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, ErrQuandoInvalido
	}
	for _, layout := range onceLayouts {
		if parsed, err := time.ParseInLocation(layout, raw, brazilLocation()); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("%w: data/hora %q", ErrQuandoInvalido, raw)
}

// NextRun calcula a próxima execução a partir de `from`. once devolve a data
// absoluta (mesmo no passado, para disparo imediato); interval soma segundos;
// cron devolve a próxima ocorrência no fuso de Brasília.
func NextRun(tipo, quando string, from time.Time) (time.Time, error) {
	if from.IsZero() {
		from = time.Now()
	}
	from = from.UTC()
	switch NormalizeTipo(tipo) {
	case TipoOnce:
		return parseOnce(quando)
	case TipoInterval:
		seconds, err := strconv.Atoi(strings.TrimSpace(quando))
		if err != nil || seconds < minIntervalSeconds {
			return time.Time{}, fmt.Errorf("%w: intervalo mínimo de %d segundos", ErrQuandoInvalido, minIntervalSeconds)
		}
		return from.Add(time.Duration(seconds) * time.Second).UTC(), nil
	case TipoCron:
		return nextCron(quando, from)
	}
	return time.Time{}, fmt.Errorf("%w: %q", ErrTipoInvalido, tipo)
}

type cronField struct {
	any        bool
	values     map[int]bool
	restricted bool
}

func (f cronField) has(value int) bool {
	if f.any {
		return true
	}
	return f.values[value]
}

func parseCronField(raw string, min, max int) (cronField, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cronField{}, ErrQuandoInvalido
	}
	field := cronField{values: map[int]bool{}, restricted: raw != "*"}
	if raw == "*" {
		field.any = true
		return field, nil
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return cronField{}, ErrQuandoInvalido
		}
		step := 1
		if slash := strings.Index(part, "/"); slash >= 0 {
			parsed, err := strconv.Atoi(strings.TrimSpace(part[slash+1:]))
			if err != nil || parsed < 1 {
				return cronField{}, ErrQuandoInvalido
			}
			step = parsed
			part = strings.TrimSpace(part[:slash])
		}
		start, end := min, max
		switch {
		case part == "*":
			start, end = min, max
		case strings.Contains(part, "-"):
			bounds := strings.SplitN(part, "-", 2)
			from, err1 := strconv.Atoi(strings.TrimSpace(bounds[0]))
			to, err2 := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err1 != nil || err2 != nil {
				return cronField{}, ErrQuandoInvalido
			}
			start, end = from, to
		default:
			value, err := strconv.Atoi(part)
			if err != nil {
				return cronField{}, ErrQuandoInvalido
			}
			start, end = value, value
		}
		if start < min || end > max || start > end {
			return cronField{}, ErrQuandoInvalido
		}
		for value := start; value <= end; value += step {
			field.values[value] = true
		}
	}
	if len(field.values) == 0 {
		return cronField{}, ErrQuandoInvalido
	}
	return field, nil
}

type cronSchedule struct {
	minute cronField
	hour   cronField
	dom    cronField
	month  cronField
	dow    cronField
}

func parseCron(expr string) (cronSchedule, error) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return cronSchedule{}, fmt.Errorf("%w: expressão cron precisa de 5 campos", ErrQuandoInvalido)
	}
	minute, err := parseCronField(fields[0], 0, 59)
	if err != nil {
		return cronSchedule{}, err
	}
	hour, err := parseCronField(fields[1], 0, 23)
	if err != nil {
		return cronSchedule{}, err
	}
	dom, err := parseCronField(fields[2], 1, 31)
	if err != nil {
		return cronSchedule{}, err
	}
	month, err := parseCronField(fields[3], 1, 12)
	if err != nil {
		return cronSchedule{}, err
	}
	dow, err := parseCronField(fields[4], 0, 7)
	if err != nil {
		return cronSchedule{}, err
	}
	if dow.values[7] {
		dow.values[0] = true
	}
	return cronSchedule{minute: minute, hour: hour, dom: dom, month: month, dow: dow}, nil
}

func (s cronSchedule) matches(t time.Time) bool {
	if !s.minute.has(t.Minute()) || !s.hour.has(t.Hour()) || !s.month.has(int(t.Month())) {
		return false
	}
	dom := s.dom.has(t.Day())
	dow := s.dow.has(int(t.Weekday()))
	switch {
	case !s.dom.restricted && !s.dow.restricted:
		return true
	case !s.dom.restricted:
		return dow
	case !s.dow.restricted:
		return dom
	default:
		return dom || dow
	}
}

func nextCron(expr string, from time.Time) (time.Time, error) {
	schedule, err := parseCron(expr)
	if err != nil {
		return time.Time{}, err
	}
	cursor := from.In(brazilLocation()).Truncate(time.Minute).Add(time.Minute)
	const maxIterations = 366 * 24 * 60 * 5
	for i := 0; i < maxIterations; i++ {
		if schedule.matches(cursor) {
			return cursor.UTC(), nil
		}
		cursor = cursor.Add(time.Minute)
	}
	return time.Time{}, fmt.Errorf("%w: nenhuma ocorrência nos próximos 5 anos", ErrQuandoInvalido)
}
