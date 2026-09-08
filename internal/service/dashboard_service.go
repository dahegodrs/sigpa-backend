package service

import (
	"context"

	"github.com/alcaldia/sigpa-backend/internal/repository"
)

type DashboardService struct {
	repo *repository.DashboardRepository
}

func NewDashboardService(repo *repository.DashboardRepository) *DashboardService {
	return &DashboardService{repo: repo}
}

// DashboardCompleto agrupa todo lo que necesita la pantalla de Dashboard Ejecutivo
// en una sola respuesta, evitando múltiples round-trips desde el frontend.
type DashboardCompleto struct {
	KPIs                   interface{}                            `json:"kpis"`
	PorDependencia         []repository.ConteoPorCampo            `json:"por_dependencia"`
	PorTipo                []repository.ConteoPorCampo            `json:"por_tipo"`
	PorEstado              []repository.ConteoPorCampo            `json:"por_estado"`
	VencimientosPorMes     []repository.ConteoPorCampo            `json:"vencimientos_por_mes"`
	VencimientosPorMesTipo []repository.VencimientoMensualPorTipo `json:"vencimientos_por_mes_tipo"`
}

func (s *DashboardService) ObtenerResumenCompleto(ctx context.Context, organizationID int) (*DashboardCompleto, error) {
	kpis, err := s.repo.ObtenerKPIs(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	porDependencia, err := s.repo.PorDependencia(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	porTipo, err := s.repo.PorTipo(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	porEstado, err := s.repo.PorEstado(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	vencimientos, err := s.repo.VencimientosPorMes(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	vencimientosPorTipo, err := s.repo.VencimientosPorMesPorTipo(ctx, organizationID)
	if err != nil {
		return nil, err
	}

	if porDependencia == nil {
		porDependencia = []repository.ConteoPorCampo{}
	}
	if porTipo == nil {
		porTipo = []repository.ConteoPorCampo{}
	}
	if porEstado == nil {
		porEstado = []repository.ConteoPorCampo{}
	}
	if vencimientos == nil {
		vencimientos = []repository.ConteoPorCampo{}
	}
	if vencimientosPorTipo == nil {
		vencimientosPorTipo = []repository.VencimientoMensualPorTipo{}
	}

	return &DashboardCompleto{
		KPIs:                   kpis,
		PorDependencia:         porDependencia,
		PorTipo:                porTipo,
		PorEstado:              porEstado,
		VencimientosPorMes:     vencimientos,
		VencimientosPorMesTipo: vencimientosPorTipo,
	}, nil
}
