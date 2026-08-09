import AppKit
import CoreGraphics
import Foundation

// Paleta Korven (lib/theme/korven_colors.dart)
func c(_ hex: UInt32, _ a: CGFloat = 1) -> CGColor {
  CGColor(srgbRed: CGFloat((hex >> 16) & 255) / 255,
          green: CGFloat((hex >> 8) & 255) / 255,
          blue: CGFloat(hex & 255) / 255, alpha: a)
}
let graphite900 = c(0x0A0E15)
let graphite850 = c(0x0E131B)
let carbon = c(0x171E29)   // graphite750
let graphite600 = c(0x283142)
let hueso = c(0xEFF3F8)    // graphite050
let accent = c(0xFF8A2B)   // amber500

/// Vértices del hexágono facetado, en el viewBox 128×128 del emblema original.
let vertices: [(CGFloat, CGFloat)] = [(64, 12), (110, 38), (110, 90), (64, 116), (18, 90), (18, 38)]
/// Las facetas van del centro a cinco vértices: (64,116) queda fuera, igual que
/// en _EmblemPainter.
let facetas: [(CGFloat, CGFloat)] = [(64, 12), (110, 38), (110, 90), (18, 90), (18, 38)]

func render(_ S: CGFloat) -> Data {
  let ctx = CGContext(data: nil, width: Int(S), height: Int(S), bitsPerComponent: 8,
                      bytesPerRow: 0, space: CGColorSpace(name: CGColorSpace.sRGB)!,
                      bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
  // Origen arriba a la izquierda, como el viewBox del emblema.
  ctx.translateBy(x: 0, y: S)
  ctx.scaleBy(x: 1, y: -1)
  ctx.setShouldAntialias(true)

  // Lienzo del icono de macOS: no va a sangre. El arte ocupa ~80 % centrado,
  // con esquinas redondeadas al 18,1 % del lado — la proporción del sistema.
  let margen = S * 0.098
  let lado = S - margen * 2
  let radio = S * 0.1809
  let placa = CGPath(roundedRect: CGRect(x: margen, y: margen, width: lado, height: lado),
                     cornerWidth: radio, cornerHeight: radio, transform: nil)

  ctx.saveGState()
  ctx.addPath(placa)
  ctx.clip()
  let degradado = CGGradient(colorsSpace: CGColorSpace(name: CGColorSpace.sRGB)!,
                             colors: [graphite850, graphite900] as CFArray,
                             locations: [0, 1])!
  ctx.drawLinearGradient(degradado, start: CGPoint(x: 0, y: margen),
                         end: CGPoint(x: 0, y: S - margen), options: [])
  ctx.restoreGState()

  // El emblema, centrado y a ~62 % del lienzo.
  let escala = (S * 0.62) / 128.0
  let dx = (S - 128 * escala) / 2
  let dy = (S - 128 * escala) / 2
  func p(_ x: CGFloat, _ y: CGFloat) -> CGPoint { CGPoint(x: dx + x * escala, y: dy + y * escala) }
  // Por debajo de ~1 px los trazos finos desaparecen: en 16 y 32 px el emblema
  // se volvía una mancha. Se les pone suelo.
  func w(_ base: CGFloat) -> CGFloat { max(base * escala, S <= 64 ? 0.9 : 1.0) }

  let hex = CGMutablePath()
  hex.move(to: p(vertices[0].0, vertices[0].1))
  for v in vertices.dropFirst() { hex.addLine(to: p(v.0, v.1)) }
  hex.closeSubpath()

  ctx.addPath(hex)
  ctx.setFillColor(carbon)
  ctx.fillPath()

  // Facetas desde el centro: el volumen tallado. Van antes del contorno para
  // que este quede limpio por encima.
  ctx.setStrokeColor(graphite600)
  ctx.setLineWidth(w(2))
  ctx.setLineJoin(.round)
  for f in facetas {
    ctx.move(to: p(64, 64))
    ctx.addLine(to: p(f.0, f.1))
  }
  ctx.strokePath()

  ctx.addPath(hex)
  ctx.setStrokeColor(hueso)
  ctx.setLineWidth(w(3))
  ctx.setLineJoin(.round)
  ctx.strokePath()

  // El ojo: halo, anillo y núcleo — el único ámbar del emblema.
  func circulo(_ r: CGFloat) -> CGRect {
    CGRect(x: p(64 - r, 64 - r).x, y: p(64 - r, 64 - r).y, width: 2 * r * escala, height: 2 * r * escala)
  }
  ctx.setFillColor(c(0xFF8A2B, 0.16)); ctx.fillEllipse(in: circulo(22))
  ctx.setStrokeColor(c(0xFF8A2B, 0.30)); ctx.setLineWidth(w(3)); ctx.strokeEllipse(in: circulo(15))
  ctx.setFillColor(accent); ctx.fillEllipse(in: circulo(9))

  let img = ctx.makeImage()!
  let rep = NSBitmapImageRep(cgImage: img)
  rep.size = NSSize(width: S, height: S)
  return rep.representation(using: .png, properties: [:])!
}

let destino = CommandLine.arguments[1]
for s in [16, 32, 64, 128, 256, 512, 1024] {
  let data = render(CGFloat(s))
  let url = URL(fileURLWithPath: "\(destino)/app_icon_\(s).png")
  try! data.write(to: url)
  print("app_icon_\(s).png  \(data.count) bytes")
}
