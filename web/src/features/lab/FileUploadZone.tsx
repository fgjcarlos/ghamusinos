import { useCallback, useRef, useState } from 'react';

/**
 * FileUploadZone: drag-drop + click-to-select para subir un archivo
 * .gpx. Sin dependencias externas — usa el `<input type="file">`
 * nativo más eventos dragenter/dragleave/drop.
 *
 * Props:
 *  - onUpload(file): callback con el File seleccionado (padre
 *    decide qué hacer: pasar al cliente API, mostrar toast, etc.)
 *  - accept: lista MIME/extensiones aceptadas (default .gpx)
 *  - disabled: deshabilita drag/click mientras el padre está subiendo
 *  - maxBytes: tope duro (default 10 MB, alineado con el body limit
 *    que el backend aplica via http.MaxBytesHandler — issue #26)
 */
export interface FileUploadZoneProps {
  onUpload: (file: File) => void;
  accept?: string;
  disabled?: boolean;
  maxBytes?: number;
}

export function FileUploadZone({
  onUpload,
  accept = '.gpx,application/gpx+xml,application/xml,text/xml',
  disabled = false,
  maxBytes = 10 * 1024 * 1024,
}: FileUploadZoneProps) {
  const [isDragging, setIsDragging] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  const handleFile = useCallback(
    (file: File) => {
      setError(null);
      if (file.size > maxBytes) {
        setError(`File exceeds ${(maxBytes / (1024 * 1024)).toFixed(1)} MB limit`);
        return;
      }
      onUpload(file);
    },
    [maxBytes, onUpload],
  );

  const onDragOver = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    if (!disabled) setIsDragging(true);
  };
  const onDragLeave = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setIsDragging(false);
  };
  const onDrop = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setIsDragging(false);
    if (disabled) return;
    const file = e.dataTransfer.files[0];
    if (file) handleFile(file);
  };
  const onClick = () => {
    if (!disabled) inputRef.current?.click();
  };
  const onChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) handleFile(file);
    // reset para permitir re-seleccionar el mismo archivo
    e.target.value = '';
  };

  return (
    <div
      onDragOver={onDragOver}
      onDragLeave={onDragLeave}
      onDrop={onDrop}
      onClick={onClick}
      className={
        disabled ? 'upload-zone disabled' : isDragging ? 'upload-zone active' : 'upload-zone'
      }
      role="button"
      tabIndex={0}
      aria-disabled={disabled}
      data-testid="gpx-upload-zone"
    >
      <input
        ref={inputRef}
        type="file"
        accept={accept}
        onChange={onChange}
        disabled={disabled}
        style={{ display: 'none' }}
      />
      {disabled ? (
        <p>Uploading…</p>
      ) : (
        <p>
          <strong>Drop a .gpx file</strong> or click to choose
          <br />
          <small>Max {Math.round(maxBytes / (1024 * 1024))} MB</small>
        </p>
      )}
      {error && <p className="error">{error}</p>}
    </div>
  );
}

export default FileUploadZone;
