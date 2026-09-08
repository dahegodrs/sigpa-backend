package service

import (
	"context"
	"time"

	"github.com/alcaldia/sigpa-backend/internal/models"
	"github.com/alcaldia/sigpa-backend/internal/repository"
)

// diasVentanaVigente es la regla del proyecto: >45 días para vencer = Vigente.
const diasVentanaVigente = 45

type DocumentoService struct {
	repo          *repository.DocumentoRepository
	historialRepo *repository.HistorialRepository
}

func NewDocumentoService(repo *repository.DocumentoRepository, historialRepo *repository.HistorialRepository) *DocumentoService {
	return &DocumentoService{repo: repo, historialRepo: historialRepo}
}

func (s *DocumentoService) ListarPorVehiculo(ctx context.Context, organizationID, vehiculoID int) ([]models.Documento, error) {
	documentos, err := s.repo.ListarPorVehiculo(ctx, organizationID, vehiculoID)
	if err != nil {
		return nil, err
	}
	// El estado se recalcula al vuelo para que la ficha del vehículo siempre
	// refleje el estado real, aunque el job diario aún no haya corrido.
	ahora := time.Now()
	for i := range documentos {
		documentos[i].EstadoDocumento = models.CalcularEstado(documentos[i].FechaVencimiento, diasVentanaVigente, ahora)
	}
	return documentos, nil
}

func (s *DocumentoService) HistoricoPorTipo(ctx context.Context, organizationID, vehiculoID, tipoDocumentoID int) ([]models.Documento, error) {
	return s.repo.HistoricoPorTipo(ctx, organizationID, vehiculoID, tipoDocumentoID)
}

// SubirNuevaVersion registra una nueva versión del documento (ej: renovación del SOAT),
// conservando automáticamente la versión anterior como histórico.
func (s *DocumentoService) SubirNuevaVersion(ctx context.Context, d *models.Documento, usuarioID int) (int, error) {
	d.EstadoDocumento = models.CalcularEstado(d.FechaVencimiento, diasVentanaVigente, time.Now())

	newID, err := s.repo.CrearNuevaVersion(ctx, d)
	if err != nil {
		return 0, err
	}

	_ = s.historialRepo.Registrar(ctx, &models.HistorialCambio{
		OrganizationID:  d.OrganizationID,
		Entidad:         "documento",
		EntidadID:       newID,
		UsuarioID:       &usuarioID,
		Accion:          "CREACION",
		CampoModificado: strPtr("archivo/version"),
	})

	return newID, nil
}

// RecalcularEstadosDiario recorre todos los documentos vigentes de todas las
// organizaciones y sincroniza su estado_documento. Pensado para ejecutarse
// como parte del job diario (ver internal/service/alerta_service.go).
func (s *DocumentoService) RecalcularEstadosDiario(ctx context.Context) (int, error) {
	documentos, err := s.repo.ListarParaRevisionDiaria(ctx)
	if err != nil {
		return 0, err
	}

	ahora := time.Now()
	actualizados := 0
	for _, d := range documentos {
		nuevoEstado := models.CalcularEstado(d.FechaVencimiento, diasVentanaVigente, ahora)
		if nuevoEstado != d.EstadoDocumento {
			if err := s.repo.ActualizarEstado(ctx, d.ID, nuevoEstado); err != nil {
				continue // se registra en logs por el caller; no se detiene el job completo
			}
			actualizados++
		}
	}
	return actualizados, nil
}

// ListarGlobal expone la Gestión Documental de todo el parque automotor.
func (s *DocumentoService) ListarGlobal(ctx context.Context, f repository.DocumentoFiltroGlobal) ([]models.Documento, int64, error) {
	return s.repo.ListarGlobal(ctx, f)
}

// ConteoPorTipo alimenta las tarjetas de "carpetas" (SOAT, Tecnomecánica, etc.)
func (s *DocumentoService) ConteoPorTipo(ctx context.Context, organizationID int) ([]repository.ConteoPorTipoDocumento, error) {
	return s.repo.ConteoPorTipo(ctx, organizationID)
}

// EliminarLogico marca el documento como eliminado y registra el evento en
// el historial de cambios, sin borrar nada de Google Drive.
func (s *DocumentoService) EliminarLogico(ctx context.Context, organizationID, documentoID, usuarioID int) error {
	if err := s.repo.EliminarLogico(ctx, organizationID, documentoID, usuarioID); err != nil {
		return err
	}

	_ = s.historialRepo.Registrar(ctx, &models.HistorialCambio{
		OrganizationID:  organizationID,
		Entidad:         "documento",
		EntidadID:       documentoID,
		UsuarioID:       &usuarioID,
		Accion:          "ELIMINACION",
		CampoModificado: strPtr("eliminado"),
	})

	return nil
}
